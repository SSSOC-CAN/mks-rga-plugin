package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SSSOC-CAN/fmtd/errors"
	sdk "github.com/SSSOC-CAN/laniakea-plugin-sdk"
	"github.com/SSSOC-CAN/laniakea-plugin-sdk/proto"
	"github.com/SSSOC-CAN/mks-rga-plugin/cfg"
	"github.com/SSSOC-CAN/mks-rga-plugin/mks"
	bg "github.com/SSSOCPaulCote/blunderguard"
	"github.com/hashicorp/go-plugin"
	influx "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/domain"
)

var (
	pluginName                               = "mks-rga-plugin"
	pluginVersion                            = "1.0.0"
	laniVersionConstraint                    = ">= 0.2.0"
	minPolInterval             time.Duration = 15 * time.Second
	ErrAlreadyRecording                      = bg.Error("already recording")
	ErrAlreadyStoppedRecording               = bg.Error("already stopped recording")
	ErrBlankInfluxOrgOrBucket                = bg.Error("influx organization or bucket cannot be blank")
	ErrInvalidOrg                            = bg.Error("invalid influx organization")
	ErrInvalidBucket                         = bg.Error("invalid influx bucket")
)

type MksRgaDatasource struct {
	sdk.DatasourceBase
	recording  int32 // used atomically
	quitChan   chan struct{}
	connection *mks.RGAConnection
	config     *cfg.Config
	client     influx.Client
	sync.WaitGroup
}

type Payload struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

type Frame struct {
	Data []Payload `json:"data"`
}

// Implements the Datasource interface funciton StartRecord
func (e *MksRgaDatasource) StartRecord() (chan *proto.Frame, error) {
	if atomic.LoadInt32(&e.recording) == 1 {
		return nil, ErrAlreadyRecording
	}
	// InitMsg
	err := e.connection.InitMsg()
	if err != nil {
		return nil, err
	}
	// Setup Data
	_, err = e.connection.Control(pluginName, pluginVersion)
	if err != nil {
		return nil, err
	}
	resp, err := e.connection.SensorState()
	if err != nil {
		return nil, err
	}
	if resp.Fields["State"].Value.(string) != mks.RGA_SENSOR_STATE_INUSE {
		return nil, fmt.Errorf("Sensor not ready: %v", resp.Fields["State"])
	}
	_, err = e.connection.AddBarchart("Bar1", 1, 200, mks.RGA_PeakCenter, 5, 0, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("Could not add Barchart: %v", err)
	}
	_, err = e.connection.ScanAdd("Bar1")
	if err != nil {
		return nil, fmt.Errorf("Could not add measurement to scan: %v", err)
	}
	var ticker *time.Ticker
	if e.config.PollingInterval == 0 || time.Duration(e.config.PollingInterval)*time.Second < minPolInterval {
		ticker = time.NewTicker(minPolInterval)
	} else {
		ticker = time.NewTicker(time.Duration(e.config.PollingInterval) * time.Second)
	}
	frameChan := make(chan *proto.Frame)
	var writeAPI api.WriteAPI
	if e.config.Influx {
		if e.config.InfluxOrgName == "" || e.config.InfluxBucketName == "" {
			return nil, ErrBlankInfluxOrgOrBucket
		}
		orgAPI := e.client.OrganizationsAPI()
		org, err := orgAPI.FindOrganizationByName(context.Background(), e.config.InfluxOrgName)
		if err != nil {
			return nil, ErrInvalidOrg
		}
		bucketAPI := e.client.BucketsAPI()
		buckets, err := bucketAPI.FindBucketsByOrgName(context.Background(), e.config.InfluxOrgName)
		if err != nil {
			return nil, ErrInvalidOrg
		}
		var found bool
		for _, bucket := range *buckets {
			if bucket.Name == e.config.InfluxBucketName {
				found = true
				break
			}
		}
		if !found {
			log.Printf("Creating %s bucket...", e.config.InfluxBucketName)
			_, err := bucketAPI.CreateBucketWithName(context.Background(), org, e.config.InfluxBucketName, domain.RetentionRule{EverySeconds: 0})
			if err != nil {
				return nil, err
			}
		}
		writeAPI = e.client.WriteAPI(e.config.InfluxOrgName, e.config.InfluxBucketName)
	}
	if ok := atomic.CompareAndSwapInt32(&e.recording, 0, 1); !ok {
		return nil, ErrAlreadyRecording
	}
	e.Add(1)
	go func() {
		defer e.Done()
		defer close(frameChan)
		defer func() {
			_, err := e.connection.FilamentControl("Off")
			if err != nil {
				log.Println(err)
			}
			_, err = e.connection.Release()
			if err != nil {
				log.Println(err)
			}
			if e.config.Influx {
				writeAPI.Flush()
				e.client.Close()
			}
			ticker.Stop()
		}()
		time.Sleep(1 * time.Second) // sleep for a second while laniakea sets up the plugin
		for {
			select {
			case <-ticker.C:
				data := []Payload{}
				df := Frame{}
				current_time := time.Now()
				// Start scan
				_, err := e.connection.ScanResume(1)
				if err != nil {
					log.Printf("Could not resume scan: %v", err)
					return
				}
				for {
					resp, err := e.connection.ReadResponse()
					if err != nil {
						log.Printf("Could not read response: %v", err)
						return
					}
					if resp.ErrMsg.CommandName == mks.MassReading {
						massPos := resp.Fields["MassPosition"].Value.(int64)
						v := resp.Fields["Value"].Value.(float64)
						data = append(data, Payload{Name: fmt.Sprintf("mass %v", massPos), Value: v})
						if e.config.Influx {
							p := influx.NewPoint(
								"pressure",
								map[string]string{
									"mass": strconv.FormatInt(massPos, 10),
								},
								map[string]interface{}{
									"pressure": v,
								},
								current_time,
							)
							// write asynchronously
							writeAPI.WritePoint(p)
						}
						if resp.Fields["MassPosition"].Value.(int64) == int64(200) {
							break
						}
					}
				}
				df.Data = data[:]
				// transform to json string
				b, err := json.Marshal(&df)
				if err != nil {
					log.Println(err)
					return
				}
				frameChan <- &proto.Frame{
					Source:    pluginName,
					Type:      "application/json",
					Timestamp: current_time.UnixMilli(),
					Payload:   b,
				}
			case <-e.quitChan:
				return
			}
		}
	}()
	return frameChan, nil
}

// Implements the Datasource interface funciton StopRecord
func (e *MksRgaDatasource) StopRecord() error {
	if ok := atomic.CompareAndSwapInt32(&e.recording, 1, 0); !ok {
		return ErrAlreadyStoppedRecording
	}
	e.quitChan <- struct{}{}
	return nil
}

// Implements the Datasource interface funciton Stop
func (e *MksRgaDatasource) Stop() error {
	close(e.quitChan)
	e.Wait()
	return nil
}

// ConnectToRGA establishes a conncetion with the RGA
func ConnectToRGA(rgaServer string) (*mks.RGAConnection, error) {
	c, err := net.Dial("tcp", rgaServer)
	if err != nil {
		return nil, err
	}
	cAssert, ok := c.(*net.TCPConn)
	if !ok {
		return nil, errors.ErrInvalidType
	}
	return &mks.RGAConnection{cAssert}, nil
}

func main() {
	config, err := cfg.InitConfig()
	if err != nil {
		log.Println(err)
		return
	}
	conn, err := ConnectToRGA(config.RGAAddr)
	if err != nil {
		log.Println(err)
		return
	}
	impl := &MksRgaDatasource{quitChan: make(chan struct{}), connection: conn, config: config}
	if config.Influx {
		if config.InfluxURL == "" || config.InfuxAPIToken == "" {
			log.Println("Influx URL or API Token config parameters cannot be blank")
		}
		impl.client = influx.NewClientWithOptions(config.InfluxURL, config.InfuxAPIToken, influx.DefaultOptions().SetTLSConfig(&tls.Config{InsecureSkipVerify: config.InfluxSkipTLS}))
	}
	impl.SetPluginVersion(pluginVersion)              // set the plugin version before serving
	impl.SetVersionConstraints(laniVersionConstraint) // set required laniakea version before serving
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: sdk.HandshakeConfig,
		Plugins: map[string]plugin.Plugin{
			pluginName: &sdk.DatasourcePlugin{Impl: impl},
		},
		// A non-nil value here enables gRPC serving for this plugin...
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
