package cfg

import (
	"io/ioutil"
	"path/filepath"

	"github.com/btcsuite/btcd/btcutil"
	yaml "gopkg.in/yaml.v2"
)

type Config struct {
	Influx           bool   `yaml:"Influx"`
	InfluxURL        string `yaml:"InfluxURL"`
	InfuxAPIToken    string `yaml:"InfluxAPIToken"`
	InfluxOrgName    string `yaml:"InfluxOrgName"`
	InfluxBucketName string `yaml:"InfluxBucketName"`
	InfluxSkipTLS    bool   `yaml:"InfluxSkipTLS"`
	RGAAddr          string `yaml:"RGAAddr"`
	PollingInterval  int64  `yaml:"PollingInterval"`
}

var configFileName = "mks.yaml"

// InitConfig initializes the config from the config YAML file
func InitConfig() (*Config, error) {
	// Use lani appdata dir for MKS plugin config
	cfgBytes, err := ioutil.ReadFile(filepath.Join(btcutil.AppDataDir("fmtd", false), configFileName))
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = yaml.Unmarshal(cfgBytes, &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
