/*
Author: Paul Côté
Last Change Author: Paul Côté
Last Date Changed: 2022/06/10
*/

package mks

import (
	"bytes"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

/*
For more information of the commands for the RGA, please visit https://mmrc.caltech.edu/Kratos%20XPS/MKS%20RGA/ASCII%20Protocol%20User%20Manual%20SP1040016.100.pdf
*/

const (
	RGA_ERROR               RGAErrStr = "ERROR"
	RGA_OK                  RGAErrStr = "OK"
	RGA_INT                 RGAType   = 0
	RGA_FLOAT               RGAType   = 1
	RGA_BOOL                RGAType   = 2
	RGA_STR                 RGAType   = 3
	RGA_SENSOR_STATE_READY            = "Ready"
	RGA_SENSOR_STATE_INUSE            = "InUse"
	RGA_SENSOR_STATE_CONFIG           = "Config"
	RGA_SENSOR_STATE_NA               = "N/A"
)

var (
	commandSuffix = "\n\r"
	delim         = []byte("\r\n")
	commandEnd    = []byte("\r\n\r\r")
	fieldRegex    = `\S+`
	/*Returns a table of sensors that can be controlled*/
	sensors = "Sensors"
	/*
		ARGS:
		    - SerialNumber: example, LM70-00197021
	*/
	selectCmd      = "Select"
	sensorState    = "SensorState"
	info           = "Info"
	eGains         = "EGains"
	inletInfo      = "InletInfo"
	rfInfo         = "RFInfo"
	multiplierInfo = "MultiplierInfo"
	/*
		ARGS:
		    - SourceIndexZero: example, 0
	*/
	sourceInfo = "SourceInfo"
	/*
		ARGS:
		    - SourceIndexZero: example, 0
	*/
	detectorInfo = "DetectorInfo"
	/*
		RESPONSE:
		    - SummaryState - Enum: OFF, WARM-UP, ON, COOL-DOWN, BAD-EMISSION
	*/
	filamentInfo = "FilamentInfo"
	/*
		RESPONSE:
		    - Pressure - In Pascals, 0 if filament is off
	*/
	totalPressureInfo = "TotalPressureInfo"
	analogInputInfo   = "AnalogInputInfo"
	analogOutputInfo  = "AnalogOutputInfo"
	digitalInfo       = "DigitalInfo"
	rolloverInfo      = "RolloverInfo"
	rVCInfo           = "RVCInfo"
	cirrusInfo        = "CirrusInfo"
	/*
		ARGS:
			- SourceIndex The 0 based index of the source parameters
			- DetectorIndex The 0 based index of the detector (0=Faraday, 1,2,3=Multiplier settings)
	*/
	pECal_Info = "PECal_Info"
	/*
		ARGS:
			- AppName String specifying the application name of the controlling application
			- Version String specifying the version of the controlling application
		EXAMPLE:
			Control "Process Eye Pro" "5.1"
	*/
	control = "Control"
	release = "Release" //defer Release()
	/*
		ARGS:
			- State Can be ‘On’ or ‘Off’
		EXAMPLE:
			FilamentControl On
	*/
	filamentControl = "FilamentControl"
	/*
		ARGS:
			- Number The filament number to select: 1 or 2
		EXAMPLE:
			FilamentSelect 2
	*/
	filamentSelect = "FilamentSelect"
	/*
		ARGS:
			- Time Number of seconds to keep the filaments on for.
		EXAMPLE:
			FilamentOnTime 200
	*/
	filamentOnTime = "FilamentOnTime"
	/*
		ARGS:
			- Name 			The name that the measurement should be called
			- StartMass 	The starting mass that should be scanned
			- EndMass 		The ending mass that should be scanned
			- PointsPerPeak Number of points to be measured across each mass
			- Accuracy 		Accuracy code to be used
			- EGainIndex 	Electronic Gain index
			- SourceIndex 	Source parameters index
			- DetectorIndex Detector parameters index
		EXAMPLE:
			AddAnalog Analog1 1 50 32 5 0 0 0
	*/
	addAnalog = "AddAnalog"
	/*
		ARGS:
			- Name 			The name that the measurement should be called
			- StartMass 	The starting mass that should be scanned
			- EndMass 		The ending mass that should be scanned
			- FilterMode 	How masses should be scanned and converted into a single reading (ENUM: PeakCenter, PeakMax, PeakAverage)
			- Accuracy 		Accuracy code to be used
			- EGainIndex 	Electronic Gain index
			- SourceIndex 	Source parameters index
			- DetectorIndex Detector parameters index
		EXAMPLE:
			AddBarchart Bar1 1 50 PeakCenter 5 0 0 0
	*/
	addBarchart = "AddBarchart"
	/*
		ARGS:
			- Name 			The name that the measurement should be called
			- FilterMode 	How masses should be scanned and converted into a single reading (ENUM: PeakCenter, PeakMax, PeakAverage)
			- Accuracy 		Accuracy code to be used
			- EGainIndex 	Electronic Gain index
			- SourceIndex 	Source parameters index
			- DetectorIndex Detector parameters index
		EXAMPLE:
			AddPeakJump PeakJump1 PeakCenter 5 0 0 0
	*/
	addPeakJump = "AddPeakJump"
	/*
		ARGS:
			- Name 			The name that the measurement should be called
			- Mass 			The mass that should be measured
			- Accuracy 		Accuracy code to be used
			- EGainIndex 	Electronic Gain index
			- SourceIndex 	Source parameters index
			- DetectorIndex Detector parameters index
		EXAMPLE:
			AddSinglePeak SinglePeak1 4.2 5 0 0 0
	*/
	addSinglePeak = "AddSinglePeak"
	/*
		ARGS:
			- Accuracy 0 – 8 Accuracy code
		EXAMPLE:
			MeasurementAccuracy 4
	*/
	measurementAccuracy = "MeasurementAccuracy"
	/*
		ARGS:
			- Mass Integer mass value
		EXAMPLE:
			MeasurementAddMass 10
	*/
	measurementAddMass = "MeasurementAddMass"
	/*
		ARGS:
			- MassIndex Index of the mass that should be changed
			- NewMass 	New mass value that should be scanned instead
		EXAMPLE:
			MeasurementChangeMass 0 6
	*/
	measurementChangeMass = "MeasurementChangeMass"
	/*
		ARGS:
			- DetectorIndex 0 based index of the detector to use for the measurement
		EXAMPLE:
			MeasurementDetectorIndex 0
	*/
	measurementDetectorIndex = "MeasurementDetectorIndex"
	/*
		ARGS:
			- EGainIndex 0 based index of the electronic gain to use for the measurement
		EXAMPLE:
			MeasurementWGainIndex 1
	*/
	measurementEGainIndex = "MeasurementWGainIndex"
	/*
		ARGS:
			- FilterMode The mode to be used to filter readings down to 1 per AMU (ENUM: PeakCenter, PeakMax, PeakAverage)
		EXAMPLE:
			MeasurementFilterMode PeakCenter
	*/
	measurementFilterMode = "MeasurementFilterMode"
	/*
		ARGS:
			- Mass The mass value to use for the selected single peak measurement. Can be fractional
		EXAMPLE:
			MeasurementMass 15.5
	*/
	measurementMass = "MeasurementMass"
	/*
		ARGS:
			- PointsPerPeak The number of points per peak to be measured for Analog measurement
		EXAMPLE:
			MeasurementPointsPerPeak 16
	*/
	measurementPointsPerPeak = "MeasurementPointsPerPeak"
	/*
		ARGS:
			- MassIndex 0 based index of the mass peak to remove from a Peak Jump measurement
		EXAMPLE:
			MeasurementRemoveMass 1
	*/
	measurementRemoveMass = "MeasurementRemoveMass"
	/*
		ARGS:
			- SourceIndex 0 based index of the source parameters to use for the measurement
		EXAMPLE:
			MeasurementSourceIndex 0
	*/
	measurementSourceIndex = "MeasurementSourceIndex"
	/*
		ARGS:
			- UseCorrection True/False whether to use rollover correction for the selected measurement
		EXAMPLE:
			MeasurementRolloverCorrection True
	*/
	measurementRolloverCorrection = "MeasurementRolloverCorrection"
	/*
		ARGS:
			- BeamOff Boolean indicating if the beam should be off during zero readings.
		EXAMPLE:
			MeasurementZeroBeamOff True
	*/
	measurementZeroBeamOff = "MeasurementZeroBeamOff"
	/*
		ARGS:
			- ZeroBufferDepth The depth of the zero reading buffer.
		EXAMPLE:
			MeasurementZeroBufferDepth 8
	*/
	measurementZeroBufferDepth = "MeasurementZeroBufferDepth"
	/*
		ARGS:
			- ZeroBufferMode The mode of operation for the zero averaging logic (ENUM: SingleScanAverage, MultiScanAverage, MultiScanAverageQuickStart, SingleShot)
		EXAMPLE:
			MeasurementZeroBufferMode MultiScanAverage
	*/
	measurementZeroBufferMode = "MeasurementZeroBufferMode"
	measurementZeroReTrigger  = "MeasurementZeroReTrigger"
	/*
		ARGS:
			- ZeroMass The mass value that should be used to take the zero readings for the measurement
		EXAMPLE:
			MeasurementZeroMass 5.5
	*/
	measurementZeroMass = "MeasurementZeroMass"
	/*
		ARGS:
			- Protect Boolean indicating if the multiplier should be locked by software.
		EXAMPLE:
			MultiplierProtect True
	*/
	multiplierProtect = "MultiplierProtect"
	runDiagnostics    = "RunDiagnostics"
	/*
		ARGS:
			- Pressure Value to be used for total pressure [Pa]
		EXAMPLE:
			TotalPressure 1.0E-4
	*/
	setTotalPressure = "TotalPressure"
	/*
		ARGS:
			- Factor Float value to apply to total pressure reading from an external gauge
		EXAMPLE:
			TotalPressureCalFactor 1.0
	*/
	totalPressureCalFactor = "TotalPressureCalFactor"
	/*
		ARGS:
			- DateTime Date in form yyyy-mm-dd_HH:MM:SS
		EXAMPLE:
			TotalPressureCalDate 2005-10-06_16:44:00
	*/
	totalPressureCalDate = "TotalPressureCalDate"
	/*
		ARGS:
			- InletOption 		How to apply inlet calibration factor (ENUM: Off, Default, Current)
			- DetectorOption 	How to apply detector calibration factor (ENUM: Off, Default, Current)
		EXAMPLE:
			CalibrationOptions Off Off
	*/
	calibrationOptions = "CalibrationOptions"
	/*
		ARGS:
			- SourceIndex 	The 0 based index of the source settings being used
			- DetectorIndex The 0 based index of the detector settings being used
			- Filament 		The filament number 1 or 2. Or 0 if both filaments factors to be set
			- Factor 		The new calibration factor
		EXAMPLE:
			DetectorFactor 0 0 1 1.5e-6
	*/
	detectorFactor = "DetectorFactor"
	/*
		ARGS:
			- SourceIndex 	The 0 based index of the source settings being used
			- DetectorIndex The 0 based index of the detector settings being used
			- Filament 		The filament number 1 or 2. Or 0 if both filaments factors to be set
			- Date 			The time and date formatted as yyyy-mm-dd_HH:MM:SS
		EXAMPLE:
			DetectorCalDate 0 0 0 2005-06-01_11:49:00
	*/
	detectorCalDate = "DetectorCalDate"
	/*
		ARGS:
			- SourceIndex 	The 0 based index of the source settings being used
			- DetectorIndex The 0 based index of the detector settings being used
			- Filament 		The filament number 1 or 2. Or 0 if both filaments factors to be set
			- Voltage 		The new multiplier voltage to use
		EXAMPLE:
			DetectorVoltage 0 1 1 500
	*/
	detectorVoltage = "DetectorVoltage"
	/*
		ARGS:
			- InletIndex 	0 based index of the inlet to set the factor for.
			- Factor 		The new inlet factor
		EXAMPLE:
			InletFactor 0 1.5
	*/
	inletFactor = "InletFactor"
	/*
		ARGS:
			- MeasurementName The measurement to add to the scan
		EXAMPLE:
			ScanAdd Analog1
	*/
	scanAdd = "ScanAdd"
	/*
		ARGS:
			- NumScans Starts a scan running and will re-trigger the scan automatically the number of times specified by NumScans
		EXAMPLE:
			ScanStart 1
	*/
	scanStart = "ScanStart"
	scanStop  = "ScanStop"
	/*
		ARGS:
			- NumScans Number of scans to re-trigger the scan for. (optional)
		EXAMPLE:
			ScanRestart
	*/
	scanResume = "ScanResume"
	/*
		ARGS:
			- NumScans Number of scans to re-trigger the scan for. (optional)
		EXAMPLE:
			ScanRestart
	*/
	scanRestart = "ScanRestart"
	/*
		ARGS:
			- MeasurementName The measurement that should be selected for other MeasurementXXXX commands
		EXAMPLE:
			MeasurementSelect Analog1
	*/
	measurementSelect = "MeasurementSelect"
	/*
		ARGS:
			- Mass The new start mass for the Analog or Barchart measurement
		EXAMPLE:
			MeasurementStartMass 50
	*/
	measurementStartMass = "MeasurementStartMass"
	/*
		ARGS:
			- Mass The new start mass for the Analog or Barchart measurement
		EXAMPLE:
			MeasurementEndMass 45
	*/
	measurementEndMass   = "MeasurementEndMass"
	measurementRemoveAll = "MeasurementRemoveAll"
	/*
		ARGS:
			- MeasurementName Name of the measurement to remove
		EXAMPLE:
			MeasurementRemove Barchart1
	*/
	measurementRemove = "MeasurementRemove"
	/*
		ARGS:
			- UseTab Boolean indicating whether to use tab characters in the output or spaces.
		EXAMPLE:
			FormatWithTab True
	*/
	formatWithTab = "FormatWithTab"
	/*
		ARGS:
			- SourceIndex 	0 based index of the source parameters entry to modify
			- IonEnergy 	New ion energy value [eV]
		EXAMPLE:
			SourceIonEnergy 0 5.5
	*/
	sourceIonEnergy = "SourceIonEnergy"
	/*
		ARGS:
			- SourceIndex 	0 based index of the source parameters entry to modify
			- Emission 		New emission value. [mA]
		EXAMPLE:
			SourceEmission 0 1.0
	*/
	sourceEmission = "SourceEmission"
	/*
		ARGS:
			- SourceIndex 	0 based index of the source parameters entry to modify
			- Extract 		New extract value. [V]
		EXAMPLE:
			SourceExtract 0 -112
	*/
	sourceExtract = "SourceExtract"
	/*
		ARGS:
			- SourceIndex 		0 based index of the source parameters entry to modify
			- ElectronEnergy 	New electron energy value [eV]
		EXAMPLE:
			SourceElectronEnergy 0 70
	*/
	sourceElectronEnergy = "SourceElectronEnergy"
	/*
		ARGS:
			- SourceIndex 		0 based index of the source parameters entry to modify
			- LowMassResolution New low mass resolution value (0 - 65535)
		EXAMPLE:
			SourceLowMassResolution 0 32767
	*/
	sourceLowMassResolution = "SourceLowMassResolution"
	/*
		ARGS:
			- SourceIndex 		0 based index of the source parameters entry to modify
			- LowMassAlignment 	New low mass alignment value (0 - 65535)
		EXAMPLE:
			SourceLowMassAlignment 0 32767
	*/
	sourceLowMassAlignment = "SourceLowMassAlignment"
	/*
		ARGS:
			- SourceIndex 		0 based index of the source parameters entry to modify
			- HighMassAlignment New high mass alignment value (0 - 65535)
		EXAMPLE:
			SourceHighMassAlignment 0 32767
	*/
	sourceHighMassAlignment = "SourceHighMassAlignment"
	/*
		ARGS:
			- SourceIndex 			0 based index of the source parameters entry to modify
			- HighMassResolution 	New high mass resolution value (0 - 65535)
		EXAMPLE:
			SourceHighMassResolution 0 32767
	*/
	sourceHighMassResolution = "SourceHighMassResolution"
	/*
		ARGS:
			- Index 			The index of the analog input (Optional)
			- NumberToAverage 	The number of readings that should be averaged before returning result (Optional)
		EXAMPLE:
			AnalogInputAverageCount
	*/
	analogInputAverageCount = "AnalogInputAverageCount"
	/*
		ARGS:
			- Index 	The index of the analog input (Optional)
			- Enable 	True/False to enable or disable the analog input (Optional)
		EXAMPLE:
			AnalogInputEnable
	*/
	analogInputEnable = "AnalogInputEnable"
	/*
		ARGS:
			- Index 	The index of the analog input (Optional)
			- Interval 	Time in microseconds between successive analog input readings [µs] (Optional)
		EXAMPLE:
			AnalogInputInterval
	*/
	analogInputInterval = "AnalogInputInterval"
	/*
		ARGS:
			- Index 	The index of the analog output (Optional)
			- Value 	The value to set the analog output to (Optional)
		EXAMPLE:
			AnalogOutput
	*/
	analogOutput = "AnalogOutput"
	/*
		ARGS:
			- Frequency The frequency in Hz to drive the sensors audio output [Hz]
		EXAMPLE:
			AudioFrequency 1000
	*/
	audioFrequency = "AudioFrequency"
	/*
		ARGS:
			- Mode The mode to run the audio in. (ENUM: Off, Automatic, Manual)
		EXAMPLE:
			AudioMode Manual
	*/
	audioMode = "AudioMode"
	/*
		ARGS:
			- HeatOn True/False to turn heater on/off
		EXAMPLE:
			CirrusCapillaryHeater False
	*/
	cirrusCapillaryHeater = "CirrusCapillaryHeater"
	/*
		ARGS:
			- Mode State to put heater into: Off, Warm or Bake
		EXAMPLE:
			CirrusHeater Warm
	*/
	cirrusHeater = "CirrusHeater"
	/*
		ARGS:
			- PumpOn True/False to turn pump On/Off
		EXAMPLE:
			CirrusPump False
	*/
	cirrusPump = "CirrusPump"
	/*
		ARGS:
			- ValvePos 0 based valve position
		EXAMPLE:
			CirrusValvePosition 1
	*/
	cirrusValvePosition = "CirrusValvePosition"
	/*
		ARGS:
			- Time Time in seconds for port B bits 6 and 7 to remain set [s]
		EXAMPLE:
			DigitalMaxPB67OnTime 600
	*/
	digitalMaxPB67OnTime = "DigitalMaxPB67OnTime"
	/*
		ARGS:
			- Port 	The port name,A, B, C, etc.
			- Value The value to set outputs to. 8 bit number (0 – 255)
		EXAMPLE:
			DigitalOutput A 192
	*/
	digitalOutput = "DigitalOutput"
	/*
		ARGS:
			- Date 		in yyyy-mm-dd_HH:MM:SS format
			- Message 	Text message to be displayed when calibration is run
		EXAMPLE:
			PECal_DateMsg 2021-10-21_10:21:00 "Y'arr maties"
	*/
	pECal_DateMsg = "PECal_DateMsg"
	pECal_Flush   = "PECal_Flush"
	/*
		ARGS:
			- Inlet1
			- Inlet2
			- Inlet3
		EXAMPLE:
			PECal_Inlet 1.0 1.0 1.0
	*/
	pECal_Inlet = "PECal_Inlet"
	/*
		ARGS:
			- Mass
			- Method
			- Contribution
		EXAMPLE:
			PECal_MassMethodContribution 28 0 80.5
	*/
	pECal_MassMethodContribution = "PECal_MassMethodContribution"
	pECal_Pressures              = "PECal_Pressures"
	/*
		ARGS:
			- SourceIndex
			- DetectorIndex
		EXAMPLE:
			PECal_Select 0 0
	*/
	pECal_Select = "PECal_Select"
	/*
		ARGS:
			- Mass 		The mass to set a specific peak scale factor for
			- Factor 	The peak scale factor for the mass
		EXAMPLE:
			RolloverScaleFactor 28 5.2
	*/
	rolloverScaleFactor = "RolloverScaleFactor"
	/*
		ARGS:
			- M1
			- M2
			- B1
			- B2
			- BP1
		EXAMPLE:
			RolloverVariables -470 -250 -0.15 -0.91 0.0012
	*/
	rolloverVariables = "RolloverVariables"
	/*
		ARGS:
			- State True/False value whether the alarm output should be set on or off.
		EXAMPLE:
			RVCAlarm True
	*/
	rVCAlarm          = "RVCAlarm"
	rVCCloseAllValves = "RVCCloseAllValves"
	/*
		ARGS:
			- HeaterOn True/False value whether to switch the heater on/off.
		EXAMPLE:
			RVCHeater True
	*/
	rVCHeater = "RVCHeater"
	/*
		ARGS:
			- PumpOn True/False value whether to switch the pump on/off.
		EXAMPLE:
			RVCPump True
	*/
	rVCPump = "RVCPump"
	/*
		ARGS:
			- Valve Index of the valve to open/close. (0 - 2)
			- Open 	True/False to open or close the valve
		EXAMPLE:
			RVCValveControl 0 True
	*/
	rVCValveControl = "RVCValveControl"
	/*
		ARGS:
			- Mode Manual or Automatic
		EXAMPLE:
			RVCValveMode Manual
	*/
	rVCValveMode = "RVCValveMode"
	saveChanges  = "SaveChanges"
	/*
		ARGS:
			- StartPower 		Percentage power to start at. Typically 10% [%]
			- EndPower 			Percentage power to ramp to. Typically 85% [%]
			- RampPeriod 		Time in seconds to ramp between StartPower and EndPower. Typically 90s [s]
			- MaxPowerPeriod 	Time to hold at EndPower. Typically 240s [s]
			- ResettlePeriod 	Time to return to default settings. Typically 30s [s]
		EXAMPLE:
			StartDegas 10 85 90 240 30
	*/
	startDegas    = "StartDegas"
	stopDegas     = "StopDegas"
	BUFFER        = 4096
	ACKMsg        = "MKSRGA"
	RGA_ERR_ERROR = func(code, msg string) error {
		return fmt.Errorf("%s\nCODE: %s\nDESCRIPTION: %s", RGA_ERROR, code, msg)
	}
	RGA_ERR_OK            = fmt.Errorf("%s", RGA_OK)
	filamentStatus        = "FilamentStatus"
	filamentTimeRemaining = "FilamentTimeRemaining"
	startingScan          = "StartingScan"
	startingMeasurement   = "StartingMeasurement"
	zeroReading           = "ZeroReading"
	MassReading           = "MassReading"
	multiplierStatus      = "MultiplierStatus"
	rfTripState           = "RFTripState"
	inletChange           = "InletChange"
	analogInput           = "AnalogInput"
	totalPressure         = "TotalPressure"
	digitalPortChange     = "DigitalPortChange"
	rvcPumpStatus         = "RVCPumpStatus"
	rvcHeaterStatus       = "RVCHeaterStatus"
	rvcValveStatus        = "RVCValveStatus"
	rvcInterlocks         = "RVCInterlocks"
	rvcStatus             = "RVCStatus"
	rvcDigitalInput       = "RVCDigitalInput"
	rvcValveMode          = "RVCValveMode"
	linkDown              = "LinkDown"
	vscEvent              = "VSCEvent"
	degasReading          = "DegasReading"
	BIG_BUFFER            = 16384
)

type RGAErrStr string

type RGAType int32

type RGAConnection struct {
	*net.TCPConn
}

var _ DriverConnectionErr = (*RGAConnection)(nil)
var _ DriverConnectionErr = RGAConnection{}

type RGARespErr struct {
	CommandName string
	Err         RGAErrStr
}
type RGAValue struct {
	Type  RGAType
	Value interface{}
}
type RGAResponse struct {
	ErrMsg RGARespErr
	Fields map[string]RGAValue
}

// String returns a string formatted RGA response
func (r *RGAResponse) String() string {
	respStr := fmt.Sprintf("%s %s\n", r.ErrMsg.CommandName, r.ErrMsg.Err)
	for field, value := range r.Fields {
		respStr += fmt.Sprintf("%s %v\n", field, value.Value)
	}
	return respStr
}

// StringSlice returns a string slice formatted RGA response
func (r *RGAResponse) StringSlice() []string {
	respSlice := []string{fmt.Sprintf("%s %s", r.ErrMsg.CommandName, r.ErrMsg.Err)}
	for field, value := range r.Fields {
		respSlice = append(respSlice, fmt.Sprintf("%s %v", field, value.Value))
	}
	return respSlice
}

// parseHorizontalResp will parse an horizontal response from the RGA into a RGAResponse object
func parseHorizontalResp(resp []byte) (*RGAResponse, error) {
	re, err := regexp.Compile(fieldRegex)
	if err != nil {
		return nil, err
	}
	split := bytes.Split(resp, delim)
	//We know the first row is always the name of the command it's error status
	errorStatusField := re.FindAllString(string(split[0]), 2)
	errorStatus := RGAErrStr(errorStatusField[1])
	var errMsg error
	if errorStatus != RGA_ERROR && errorStatus != RGA_OK {
		return nil, fmt.Errorf("Unkown RGA error code: %s", errorStatus)
	} else if errorStatus == RGA_ERROR {
		var errDescription string
		splitAgain := re.FindAllString(string(split[2]), -1)
		for i := 1; i < len(splitAgain); i++ {
			errDescription += splitAgain[i] + " "
		}
		return nil, RGA_ERR_ERROR(re.FindAllString(string(split[1]), 2)[1], errDescription)
	}
	// We know that the second row will be headers
	headers := re.FindAllString(string(split[1]), -1)
	// We know that split has a length of four since it's an horizontal response. We can ignore the last line.
	fields := make(map[string]RGAValue)
	for j := 2; j < len(split)-1; j++ {
		values := re.FindAllString(string(split[j]), len(headers))
		for i, header := range headers {
			if j > 2 {
				header = header + strconv.Itoa(i-2)
			}
			if _, ok := fields[header]; !ok {
				// if int64
				if v, err := strconv.ParseInt(values[i], 10, 64); err == nil {
					fields[header] = RGAValue{Type: RGA_INT, Value: v}
					// if float64
				} else if v, err := strconv.ParseFloat(values[i], 64); err == nil {
					fields[header] = RGAValue{Type: RGA_FLOAT, Value: v}
					// if bool
				} else if v, err := strconv.ParseBool(values[i]); err == nil {
					fields[header] = RGAValue{Type: RGA_BOOL, Value: v}
					// if string
				} else {
					fields[header] = RGAValue{Type: RGA_STR, Value: values[i]}
				}
			}
		}
	}

	return &RGAResponse{
		ErrMsg: RGARespErr{
			CommandName: errorStatusField[0],
			Err:         errorStatus,
		},
		Fields: fields,
	}, errMsg
}

// parseVerticalResp will parse a vertical response from the RGA into a RGAResponse object
func parseVerticalResp(resp []byte, oneValuePerLine bool) (*RGAResponse, error) {
	re, err := regexp.Compile(fieldRegex)
	if err != nil {
		return nil, err
	}
	split := bytes.Split(resp, delim)
	//We know the first row is always the name of the command it's error status
	errorStatusField := re.FindAllString(string(split[0]), 2)
	errorStatus := RGAErrStr(errorStatusField[1])
	if errorStatus != RGA_ERROR && errorStatus != RGA_OK {
		return nil, fmt.Errorf("Unkown RGA error code: %s", errorStatus)
	} else if errorStatus == RGA_ERROR {
		var errDescription string
		splitAgain := re.FindAllString(string(split[2]), -1)
		for i := 1; i < len(splitAgain); i++ {
			errDescription += splitAgain[i] + " "
		}
		return nil, RGA_ERR_ERROR(re.FindAllString(string(split[1]), 2)[1], errDescription)
	}
	// We know that the second to the before last will be our header value combos
	fields := make(map[string]RGAValue)
	for i := 1; i < len(split)-1; i++ {
		var (
			field []string
			name  string
			value string
		)
		if oneValuePerLine {
			field = re.FindAllString(string(split[i]), 2)
			name = fmt.Sprintf("Value%d", i)
			value = field[0]
		} else {
			field = re.FindAllString(string(split[i]), 2)
			name = field[0]
			value = field[1]
		}
		if _, ok := fields[field[0]]; !ok {
			// if int64
			if v, err := strconv.ParseInt(value, 10, 64); err == nil {
				fields[name] = RGAValue{Type: RGA_INT, Value: v}
				// if float64
			} else if v, err := strconv.ParseFloat(value, 64); err == nil {
				fields[name] = RGAValue{Type: RGA_FLOAT, Value: v}
				// if bool
			} else if v, err := strconv.ParseBool(value); err == nil {
				fields[name] = RGAValue{Type: RGA_BOOL, Value: v}
				// if string
			} else {
				fields[name] = RGAValue{Type: RGA_STR, Value: value}
			}
		}
	}
	return &RGAResponse{
		ErrMsg: RGARespErr{
			CommandName: errorStatusField[0],
			Err:         errorStatus,
		},
		Fields: fields,
	}, nil
}

// InitMsg returns the standard response when the RGA receives it's first message from the client
func (c *RGAConnection) InitMsg() error {
	fmt.Fprintf(c, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	split := bytes.Split(resp[0], delim)
	splitAgain := bytes.Split(split[0], delim)
	re, err := regexp.Compile(fieldRegex)
	if err != nil {
		return err
	}
	firstLine := re.FindAllString(string(splitAgain[0]), 2)
	if firstLine[0] != ACKMsg {
		return fmt.Errorf("RGA did not respond with expected ACK msg: %s", string(split[0]))
	}
	return nil
}

//ReadResponse reads an asynchronous response from the RGA
func (c *RGAConnection) ReadResponse() (*RGAResponse, error) {
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	//Parse response here
	re, err := regexp.Compile(fieldRegex)
	if err != nil {
		return nil, err
	}
	split := bytes.Split(resp[0], delim)
	// We first ensure we are parsing a mass response
	firstRow := re.FindAllString(string(split[0]), -1)
	fields := make(map[string]RGAValue)
	var headers []string
	switch firstRow[0] {
	case startingScan:
		headers = []string{"ScanNumber", "Time", "ScansRemaining"}
	case startingMeasurement:
		headers = []string{"MeasurementName"}
	case zeroReading:
		headers = []string{"MassPosition", "Value"}
	case MassReading:
		headers = []string{"MassPosition", "Value"}
	case filamentTimeRemaining:
		headers = []string{"Time"}
	case multiplierStatus:
		var trueResp []byte
		trueResp = append(trueResp, []byte(multiplierStatus+" OK")...)
		trueResp = append(trueResp, resp[0]...)
		return parseVerticalResp(trueResp, false)
	case rfTripState:
		headers = []string{"State"}
	case inletChange:
		headers = []string{"Index"}
	case analogInput:
		headers = []string{"Index", "Value"}
	case totalPressure:
		headers = []string{"Value"}
	case digitalPortChange:
		headers = []string{"Port", "Value"}
	case linkDown:
		headers = []string{"Reason"}
	case vscEvent:
		var trueResp []byte
		trueResp = append(trueResp, []byte(vscEvent+" OK")...)
		for i := 1; i < len(split); i++ {
			trueResp = append(trueResp, split[i]...)
		}
		return parseVerticalResp(trueResp, false)
	case degasReading:
		var trueResp []byte
		trueResp = append(trueResp, []byte(degasReading+" OK")...)
		trueResp = append(trueResp, resp[0]...)
		return parseVerticalResp(trueResp, false)
	}
	i := 1
	for _, name := range headers {
		if _, ok := fields[name]; !ok {
			// if int64
			if v, err := strconv.ParseInt(firstRow[i], 10, 64); err == nil {
				fields[name] = RGAValue{Type: RGA_INT, Value: v}
				// if float64
			} else if v, err := strconv.ParseFloat(firstRow[i], 64); err == nil {
				fields[name] = RGAValue{Type: RGA_FLOAT, Value: v}
				// if bool
			} else if v, err := strconv.ParseBool(firstRow[i]); err == nil {
				fields[name] = RGAValue{Type: RGA_BOOL, Value: v}
				// if string
			} else {
				fields[name] = RGAValue{Type: RGA_STR, Value: firstRow[i]}
			}
		}
		i++
	}
	return &RGAResponse{
		ErrMsg: RGARespErr{
			CommandName: firstRow[0],
			Err:         RGA_OK,
		},
		Fields: fields,
	}, nil
}

// Sensors returns a table of sensors that can be controlled
func (c *RGAConnection) Sensors() (*RGAResponse, error) {
	fmt.Fprintf(c, sensors+commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseHorizontalResp(resp[0])
}

// Select selects the device via the inputted SerialNumber
func (c *RGAConnection) Select(SerialNumber string) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %s%s", selectCmd, SerialNumber, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// SensorState retrieves the state of the selected sensor. State can only one of Ready, InUse, Config, N/A
func (c *RGAConnection) SensorState() (*RGAResponse, error) {
	fmt.Fprintf(c, sensorState+commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// Info returns the sensor config
func (c *RGAConnection) Info() (*RGAResponse, error) {
	fmt.Fprintf(c, info+commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// EGains returns the list of electronic gain factors available for the sensor
func (c *RGAConnection) EGains() (*RGAResponse, error) {
	fmt.Fprintf(c, eGains+commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], true)
}

// InletInfo returns inlet information
func (c *RGAConnection) InletInfo() (*RGAResponse, error) {
	fmt.Fprintf(c, inletInfo+commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseHorizontalResp(resp[0])
}

// RFInfo returns the current configuration and state of the RF Trip
func (c *RGAConnection) RFInfo() (*RGAResponse, error) {
	fmt.Fprintf(c, rfInfo+commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// MultiplierInfo returns the current configuration the current state of the multiplier and the reason why it is locked
func (c *RGAConnection) MultiplierInfo() (*RGAResponse, error) {
	fmt.Fprintf(c, multiplierInfo+commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// SourceInfo returns the current configuration the current state of the multiplier and the reason why it is locked
func (c *RGAConnection) SourceInfo() (*RGAResponse, error) {
	fmt.Fprintf(c, sourceInfo+commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// DetectorInfo returns a table of information about the detector settings for a particular source table
func (c *RGAConnection) DetectorInfo(SourceIndex int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d%s", detectorInfo, SourceIndex, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	// Detector info has a line that is vertical so we parse that separately
	split := bytes.Split(resp[0], delim)
	var vertBytes []byte
	vertBytes = append(vertBytes, split[0]...)
	vertBytes = append(vertBytes, delim...)
	vertBytes = append(vertBytes, split[1]...)
	vertBytes = append(vertBytes, delim...)
	vertPortion, err := parseVerticalResp(vertBytes, false)
	if err != nil {
		return nil, err
	}
	var horizBytes []byte
	horizBytes = append(horizBytes, split[0]...)
	horizBytes = append(horizBytes, delim...)
	for i := 2; i < len(split)-1; i++ {
		horizBytes = append(horizBytes, split[i]...)
		horizBytes = append(horizBytes, delim...)
	}
	horizPortion, err := parseHorizontalResp(horizBytes)
	if err != nil {
		return nil, err
	}
	for name, value := range vertPortion.Fields {
		horizPortion.Fields[name] = value
	}
	return horizPortion, nil
}

// FilamentInfo returns the current config and state of the filaments
func (c *RGAConnection) FilamentInfo() (*RGAResponse, error) {
	fmt.Fprintf(c, filamentInfo+commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// TotalPressureInfo returns information about the current state and settings being used if a total pressure gauge has been fitted onto the sensor
func (c *RGAConnection) TotalPressureInfo() (*RGAResponse, error) {
	fmt.Fprintf(c, totalPressureInfo+commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// AnalogInputInfo returns information about all the analog inputs that the sensor has.
func (c *RGAConnection) AnalogInputInfo() (*RGAResponse, error) {
	fmt.Fprintf(c, analogInputInfo+commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseHorizontalResp(resp[0])
}

// AnalogOutputInfo rReturns information about all analog outputs that a sensor has
func (c *RGAConnection) AnalogOutputInfo() (*RGAResponse, error) {
	fmt.Fprintf(c, analogOutputInfo+commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseHorizontalResp(resp[0])
}

// DigitalInfo returns information about the fitted digital input ports
func (c *RGAConnection) DigitalInfo() (*RGAResponse, error) { // TODO:SSSOCPaulCote this command has both vertical and horizontal outputs
	fmt.Fprintf(c, digitalInfo+commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// RolloverInfo Returns configuration settings for the rollover correction algorithm used in the HPQ2s
func (c *RGAConnection) RolloverInfo() (*RGAResponse, error) {
	fmt.Fprintf(c, rolloverInfo+commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// RVCInfo returns the current state of the RVC if the sensor has an RVC fitted.
func (c *RGAConnection) RVCInfo() (*RGAResponse, error) {
	fmt.Fprintf(c, rVCInfo+commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

//CirrusInfo returns the current Cirrus status and configuration if the sensor is a Cirrus
func (c *RGAConnection) CirrusInfo() (*RGAResponse, error) {
	fmt.Fprintf(c, cirrusInfo+commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

//PECal_Info It is not meant to be used by non MKS software
func (c *RGAConnection) PECal_Info(SourceIndex, DetectorIndex int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d %d%s", pECal_Info, SourceIndex, DetectorIndex, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

//Control takes an AppName (name of the TCP client) and the version of the controlling application to control an unused sensor
func (c *RGAConnection) Control(AppName, Version string) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %s %s%s", control, AppName, Version, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// Release releases control of the sensor
func (c *RGAConnection) Release() (*RGAResponse, error) {
	fmt.Fprintf(c, release+commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

type RGAOnOff string

const (
	RGA_ON  RGAOnOff = "On"
	RGA_OFF RGAOnOff = "Off"
)

// FilamentControl turns the currently selected filament On or Off
func (c *RGAConnection) FilamentControl(State RGAOnOff) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %s%s", filamentControl, State, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// FilamentSelect selects a particular filament
func (c *RGAConnection) FilamentSelect(Number int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d%s", filamentSelect, Number, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// FilamentOnTime sets the amount of time that filaments will stay on for if the unit is configured to use a time limit before filaments automatically go off
func (c *RGAConnection) FilamentOnTime(Time int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d%s", filamentOnTime, Time, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// AddAnalog adds a new analog measurement to the sensor
func (c *RGAConnection) AddAnalog(Name string, StartMass, EndMass, PointerPerPeak, Accuracy, SourceIndex, DetectorIndex int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %s %d %d %d %d %d %d%s", addAnalog, Name, StartMass, EndMass, PointerPerPeak, Accuracy, SourceIndex, DetectorIndex, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

type RGAFilterMode string

const (
	RGA_PeakCenter  RGAFilterMode = "PeakCenter"
	RGA_PeakMax     RGAFilterMode = "PeakMax"
	RGA_PeakAverage RGAFilterMode = "PeakAverage"
)

// AddBarchart adds a new barchart measurement to the sensor
func (c *RGAConnection) AddBarchart(Name string, StartMass, EndMass int, FilterMode RGAFilterMode, Accuracy, EGainIndex, SourceIndex, DetectorIndex int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %s %d %d %s %d %d %d %d%s", addBarchart, Name, StartMass, EndMass, FilterMode, Accuracy, EGainIndex, SourceIndex, DetectorIndex, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// AddPeakJump adds a new peak jump measurement to the sensor
func (c *RGAConnection) AddPeakJump(Name string, FilterMode RGAFilterMode, Accuracy, EGainIndex, SourceIndex, DetectorIndex int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %s %s %d %d %d %d%s", addPeakJump, Name, FilterMode, Accuracy, EGainIndex, SourceIndex, DetectorIndex, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// AddSinglePeak adds a new single peak measurement to the sensor
func (c *RGAConnection) AddSinglePeak(Name string, Mass float64, Accuracy, EGainIndex, SourceIndex, DetectorIndex int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %s %f %d %d %d %d%s", addSinglePeak, Name, Mass, Accuracy, EGainIndex, SourceIndex, DetectorIndex, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// MeasurementAccuracy changes the accuracy code of the currently selected measurement.
func (c *RGAConnection) MeasurementAccuracy(Accuracy int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d%s", measurementAccuracy, Accuracy, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// MeasurementAddMass adds a mass to a peak jump measurement
func (c *RGAConnection) MeasurementAddMass(Mass int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d%s", measurementAddMass, Mass, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// MeasurementChangeMass changes a mass on a Peak Jump measurement
func (c *RGAConnection) MeasurementChangeMass(MassIndex, NewMass int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d %d%s", measurementChangeMass, MassIndex, NewMass, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// MeasurmentDetectorIndex changes the selected measurements detector index
func (c *RGAConnection) MeasurementDetectorIndex(DetectorIndex int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d%s", measurementDetectorIndex, DetectorIndex, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// MeasurementEGainIndex changes a measurements electronic gain index
func (c *RGAConnection) MeasurementEGainIndex(EGainIndex int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d%s", measurementEGainIndex, EGainIndex, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// MeasurementFilterMode selects the mass filter mode to be used for the Barchart and Peak Jump measurements
func (c *RGAConnection) MeasurementFilterMode(FilterMode RGAFilterMode) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %s%s", measurementFilterMode, FilterMode, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// MeasurementMass changes the mass used for the selected single peak measurement
func (c *RGAConnection) MeasurementMass(Mass float64) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %f%s", measurementMass, Mass, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// MeasurementPointsPerPeak sets the selected analog measurements number of points to measure per peak (or AMU)
func (c *RGAConnection) MeasurementPointsPerPeak(PointsPerPeak int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d%s", measurementPointsPerPeak, PointsPerPeak, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// MeasurementRemoveMass removes a mass peak from the selected Peak Jump measurement
func (c *RGAConnection) MeasurementRemoveMass(MassIndex int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d%s", measurementRemoveMass, MassIndex, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// MeasurementSourceIndex changes the selected measurements source parameters
func (c *RGAConnection) MeasurementSourceIndex(SourceIndex int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d%s", measurementSourceIndex, SourceIndex, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// MeasurementRolloverCorrection changes whether the selected measurement uses rollover correction
func (c *RGAConnection) MeasurementRolloverCorrection(UseCorrection bool) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %s%s", measurementRolloverCorrection, strings.Title(fmt.Sprintf("%t", UseCorrection)), commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// MeasurementZeroBeamOff controls whether the ion beam should be on or off during a measurements zero readings
func (c *RGAConnection) MeasurementZeroBeamOff(BeamOff bool) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %s%s", measurementZeroBeamOff, strings.Title(fmt.Sprintf("%t", BeamOff)), commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// MeasurementZeroBufferDepth sets the selected measurements zero buffer depth
func (c *RGAConnection) MeasurementZeroBufferDepth(ZeroBufferDepth int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d%s", measurementZeroBufferDepth, ZeroBufferDepth, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

type RGAZeroBufferMode string

const (
	RGA_SingleScanAverage          RGAZeroBufferMode = "SingleScanAverage"
	RGA_MultiScanAverage           RGAZeroBufferMode = "MultiScanAverage"
	RGA_MultiScanAverageQuickStart RGAZeroBufferMode = "MultiScanAverageQuickStart"
	RGA_SingleShot                 RGAZeroBufferMode = "SingleShot"
)

// MeasurementZeroBufferMode sets the selected measurements zero buffer mode
func (c *RGAConnection) MeasurementZeroBufferMode(ZeroBufferMode RGAZeroBufferMode) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %s%s", measurementZeroBufferMode, ZeroBufferMode, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// MeasurementZeroReTrigger re-triggers the selected measurements zero buffer if it's mode is SingleShot
func (c *RGAConnection) MeasurementZeroReTrigger() (*RGAResponse, error) {
	fmt.Fprintf(c, measurementZeroReTrigger+commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// MeasurementZeroMass sets the mass position where the selected measurement should take it's zero readings from
func (c *RGAConnection) MeasurementZeroMass(ZeroMass float64) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %f%s", measurementZeroMass, ZeroMass, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// MultiplierProtect controls whether the multiplier is allowed to come on or not
func (c *RGAConnection) MultiplierProtect(Protect bool) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %s%s", multiplierProtect, strings.Title(fmt.Sprintf("%t", Protect)), commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// RunDiagnostics runs the sensors diagnostics measurements
func (c *RGAConnection) RunDiagnostics() (*RGAResponse, error) {
	fmt.Fprintf(c, runDiagnostics+commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseHorizontalResp(resp[0])
}

// SetTotalPressure if no gauge is fitted for measuring total pressure it is sometimes useful to pass in a value for total pressure so that the sensors roll over correction can still function properly
func (c *RGAConnection) SetTotalPressure(Pressure float64) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %E%s", setTotalPressure, Pressure, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// TotalPressureCalFactor sets a value to apply to external gauge total pressure readings to compensate for any differences between the gauge and the true pressure
func (c *RGAConnection) TotalPressureCalFactor(Factor float64) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %f%s", totalPressureCalFactor, Factor, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// TotalPressureCalDate sets the date/time associated with a calibration
func (c *RGAConnection) TotalPressureCalDate(DateTime time.Time) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d-%2d-%2d_%2d:%2d:%2d%s", totalPressureCalDate, DateTime.Year(), DateTime.Month(), DateTime.Day(), DateTime.Hour(), DateTime.Minute(), DateTime.Second(), commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

type RGAOption string

const (
	RGA_OPTION_OFF     RGAOption = "Off"
	RGA_OPTION_DEFAULT RGAOption = "Default"
	RGA_OPTION_CURRENT RGAOption = "Current"
)

// CalibrationOptions sets how to apply calibration factors to acquired measurement data
func (c *RGAConnection) CalibrationOptions(InletOption, DetectorOption RGAOption) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %s %s%s", calibrationOptions, InletOption, DetectorOption, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// DetectorFactor sets a calibration factor for a given set of source parameters and detector parameters
func (c *RGAConnection) DetectorFactor(SourceIndex, DetectorIndex, Filament int, Factor float64) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d %d %d %e%s", detectorFactor, SourceIndex, DetectorIndex, Filament, Factor, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// DetectorCalDate sets a calibration date for a given set of source parameters and detector parameters
func (c *RGAConnection) DetectorCalDate(SourceIndex, DetectorIndex, Filament int, Date time.Time) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d %d %d %d-%2d-%2d_%2d:%2d:%2d%s", detectorCalDate, SourceIndex, DetectorIndex, Filament, Date.Year(), Date.Month(), Date.Day(), Date.Hour(), Date.Minute(), Date.Second(), commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// DetectorVoltage sets the multiplier voltage for a particular set of detector settings
func (c *RGAConnection) DetectorVoltage(SourceIndex, DetectorIndex, Filament, Voltage int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d %d %d %d%s", detectorVoltage, SourceIndex, DetectorIndex, Filament, Voltage, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// InletFactor sets a particular inlets pressure reduction factor
func (c *RGAConnection) InletFactor(InletIndex int, Factor float64) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d %f%s", inletFactor, InletIndex, Factor, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// ScanAdd adds a measurement to the scans list of measurements
func (c *RGAConnection) ScanAdd(MeasurementName string) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %s%s", scanAdd, MeasurementName, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// ScanStart starts a scan running and will re-trigger the scan automatically the number of times specified by NumScans
func (c *RGAConnection) ScanStart(NumScans int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d%s", scanStart, NumScans, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// ScanStop stops a scan and removes all measurements from the scan list
func (c *RGAConnection) ScanStop() (*RGAResponse, error) {
	fmt.Fprintf(c, scanStop+commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// ScanResume re-triggers the scan NumScans times
func (c *RGAConnection) ScanResume(NumScans int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d%s", scanResume, NumScans, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// ScanRestart re-starts the current scan from the beginning
func (c *RGAConnection) ScanRestart() (*RGAResponse, error) {
	fmt.Fprintf(c, "%s%s", scanRestart, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// MeasurementSelect selects a measurement for other MeasurementXXX comands to act upon
func (c *RGAConnection) MeasurementSelect(MeasurementName string) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %s%s", measurementSelect, MeasurementName, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// MeasurementStartMass sets the selected Analog or Barchart measurements starting mass
func (c *RGAConnection) MeasurmentStartMass(Mass int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d%s", measurementStartMass, Mass, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// MeasurementEndMass sets the selected analog or barchart measurements ending mass
func (c *RGAConnection) MeasurementEndMass(Mass int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d%s", measurementEndMass, Mass, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// MeasurementRemoveAll removes all measurements from the sensor
func (c *RGAConnection) MeasurementRemoveAll() (*RGAResponse, error) {
	fmt.Fprintf(c, measurementRemoveAll+commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// MeasurementRemove removes the specified measurement from the sensor
func (c *RGAConnection) MeasurementRemove(MeasurementName string) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %s%s", measurementRemove, MeasurementName, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// FormatWithTab by default the output from commands is formatted using spaces to try to line everything up
// when output using a fixed width font (or terminal program). By sending this command clients can reduce the amount
// of characters sent in each message slightly as groups of spaces will be replaced by a single tab character
func (c *RGAConnection) FormatWithTab(UseTab bool) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %s%s", formatWithTab, strings.Title(fmt.Sprintf("%t", UseTab)), commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// SourceIonEnergy sets a source settings parameters Ion Energy setting
func (c *RGAConnection) SourceIonEnergy(SourceIndex int, IonEnergy float64) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d %f%s", sourceIonEnergy, SourceIndex, IonEnergy, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// SourceEmission sets a source settings parameters Emission setting
func (c *RGAConnection) SourceEmission(SourceIndex int, Emission float64) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d %f%s", sourceEmission, SourceIndex, Emission, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// SourceExtract sets a source settings parameters Extract setting
func (c *RGAConnection) SourceExtract(SourceIndex, Extract int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d %d%s", sourceExtract, SourceIndex, Extract, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// SourceElectronEnergy sets a source settings parameters Electron Energy setting
func (c *RGAConnection) SourceElectronEnergy(SourceIndex, ElectronEnergy int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d %d%s", sourceElectronEnergy, SourceIndex, ElectronEnergy, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// SourceLowMassResolution sets a source settings parameters Low Mass Resolution setting
func (c *RGAConnection) SourceLowMassResolution(SourceIndex, LowMassResolution int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d %d%s", sourceLowMassResolution, SourceIndex, LowMassResolution, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// SourceLowMassAlignment sets a source settings parameters Low Mass Alignment setting
func (c *RGAConnection) SourceLowMassAlignment(SourceIndex, LowMassAlignment int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d %d%s", sourceLowMassAlignment, SourceIndex, LowMassAlignment, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// SourceHighMassAlignment sets a source settings parameters High Mass Alignment setting
func (c *RGAConnection) SourceHighMassAlignment(SourceIndex, HighMassAlignment int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d %d%s", sourceHighMassAlignment, SourceIndex, HighMassAlignment, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// SourceHighMassResolution sets a source settings parameters High Mass Resolution setting
func (c *RGAConnection) SourceHighMassResolution(SourceIndex, HighMassResolution int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d %d%s", sourceHighMassResolution, SourceIndex, HighMassResolution, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// AnalogInputAverageCount sets the number of readings that should be taken and averaged before results are sent back
func (c *RGAConnection) AnalogInputAverageCount(Index, NumberToAverage int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d %d%s", analogInputAverageCount, Index, NumberToAverage, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// AnalogInputEnable enables or disables analog input readings from being sent when in control of the sensor
func (c *RGAConnection) AnalogInputEnable(Index int, Enable bool) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d %s%s", analogInputEnable, Index, strings.Title(fmt.Sprintf("%t", Enable)), commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// AnalogInputInterval sets the interval between analog input readings in the sensor
func (c *RGAConnection) AnalogInputInterval(Index, Interval int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d %d%s", analogInputInterval, Index, Interval, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// AnalogOutput sets a given analog output channel to the specified voltage
func (c *RGAConnection) AnalogOutput(Index, Value int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d %d%s", analogOutput, Index, Value, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// AudioFrequency sets the frequency of the audio output if the sensor supports audio output
func (c *RGAConnection) AudioFrequency(Frequency int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d%s", audioFrequency, Frequency, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

type RGAAudioMode string

const (
	RGA_AUDIO_OFF       RGAAudioMode = "Off"
	RGA_AUDIO_AUTOMATIC RGAAudioMode = "Automatic"
	RGA_AUDIO_MANUAL    RGAAudioMode = "Manual"
)

// AudioMode changes the mode if the sensor supports audio output
func (c *RGAConnection) AudioMode(Mode RGAAudioMode) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %s%s", audioMode, Mode, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// CirrusCapillaryHeater turns the capillary heater on/off
func (c *RGAConnection) CirrusCapillaryHeater(HeatOn bool) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %s%s", cirrusCapillaryHeater, strings.Title(fmt.Sprintf("%t", HeatOn)), commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

type RGACirrusHeaterMode string

const (
	RGA_CIRRUS_HEATER_OFF  RGACirrusHeaterMode = "Off"
	RGA_CIRRUS_HEATER_WARM RGACirrusHeaterMode = "Warm"
	RGA_CIRRUS_HEATER_BAKE RGACirrusHeaterMode = "Bake"
)

// CirrusHeater sets the cirrus heater into the mode requested
func (c *RGAConnection) CirrusHeater(Mode RGACirrusHeaterMode) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %s%s", cirrusHeater, Mode, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// CirrusPump turns the cirrus pumps on or off
func (c *RGAConnection) CirrusPump(PumpOn bool) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %s%s", cirrusPump, strings.Title(fmt.Sprintf("%t", PumpOn)), commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// CirrusValvePosition moves the cirrus rotary valve to the specified position
func (c *RGAConnection) CirrusValvePosition(ValvePos int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d%s", cirrusValvePosition, ValvePos, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// DigitalMaxPB67OnTime sets the time that either pin 6 or 7 will remain set for after they are initially set
func (c *RGAConnection) DigitalMaxPB67OnTime(Time int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d%s", digitalMaxPB67OnTime, Time, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// DigitalOutput sets digital outputs according to the value specified
func (c *RGAConnection) DigitalOutput(Port string, Value int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %s %d%s", digitalOutput, Port, Value, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// PECal_DateMsg this is a message specifically for MKS Process Eye software to maintain compatability with some of the softwares calibration features
func (c *RGAConnection) PECal_DateMsg(Date time.Time, Message string) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d-%2d-%2d_%2d:%2d:%2d %s%s", pECal_DateMsg, Date.Year(), Date.Month(), Date.Day(), Date.Hour(), Date.Minute(), Date.Second(), Message, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// PECal_Flush tThis is a message specifically for MKS Process Eye software to maintain compatability with some of the softwares
// calibration features. It flushes the selected calibration settings to persistent storeage of the sensor.
func (c *RGAConnection) PECal_Flush() (*RGAResponse, error) {
	fmt.Fprintf(c, pECal_Flush+commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// PECal_Inlet this is a message specifically for MKS Process Eye software to maintain compatability with some of the software calibration features.
func (c *RGAConnection) PECal_Inlet(Inlet1, Inlet2, Inlet3 float64) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %f %f %f%s", pECal_Inlet, Inlet1, Inlet2, Inlet3, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// PECal_MassMethodContribution this is a message specifically for MKS Process Eye software to maintain compatability with some of the softwares calibration features
func (c *RGAConnection) PECal_MassMethodContribution(Mass, Method int, Contribution float64) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d %d %f%s", pECal_MassMethodContribution, Mass, Method, Contribution, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// PECal_Pressures this is a message specifically for MKS Process Eye software to maintain compatability with some of the softwares calibration features
func (c *RGAConnection) PECal_Pressures() (*RGAResponse, error) {
	fmt.Fprintf(c, pECal_Pressures+commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// PECal_Select This is a message specifically for MKS Process Eye software to maintain compatability with some of the softwares calibration features
func (c *RGAConnection) PECal_Select(SourceIndex, DetectorIndex int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d %d%s", pECal_Select, SourceIndex, DetectorIndex, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// RolloverScaleFactor sets a given masses peak scale factor to compensate for differences in sensitivity to the rollover effect
func (c *RGAConnection) RolloverScaleFactor(Mass int, Factor float64) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d %f%s", rolloverScaleFactor, Mass, Factor, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// RolloverVariables sets the key rollover algorithm constants
func (c *RGAConnection) RolloverVariables(M1, M2 int, B1, B2, BP1 float64) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d %d %f %f %f%s", rolloverVariables, M1, M2, B1, B2, BP1, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// RVCAlarm sets of clears the digital alarm output on the RVC
func (c *RGAConnection) RVCAlarm(State bool) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %s%s", rVCAlarm, strings.Title(fmt.Sprintf("%t", State)), commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// RVCCloseAllValves the sensor must have an RVC fitted for this command to be available.
func (c *RGAConnection) RVCCloseAllValves() (*RGAResponse, error) {
	fmt.Fprintf(c, rVCCloseAllValves+commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// RVCHeater turns the RVC heater on/off
func (c *RGAConnection) RVCHeater(HeaterOn bool) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %s%s", rVCHeater, strings.Title(fmt.Sprintf("%t", HeaterOn)), commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// RVCPump turns the pump on or off
func (c *RGAConnection) RVCPump(PumpOn bool) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %s%s", rVCPump, strings.Title(fmt.Sprintf("%t", PumpOn)), commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// RVCValveControl opens or closes a specific valve
func (c *RGAConnection) RVCValveControl(Valve int, Open bool) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d %s%s", rVCValveControl, Valve, strings.Title(fmt.Sprintf("%t", Open)), commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

type RGARVCValveMode string

const (
	RGA_RVC_VALVE_MANUAL    RGARVCValveMode = "Manual"
	RGA_RVC_VALVE_AUTOMATIC RGARVCValveMode = "Automatic"
)

// RVCValveMode switches valve mode between manual and automatic mode
func (c *RGAConnection) RVCValveMode(Mode RGARVCValveMode) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %s%s", rVCValveMode, Mode, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// SaveChanges saves any changes that may have been made to the tuning/calibration of the sensor
func (c *RGAConnection) SaveChanges() (*RGAResponse, error) {
	fmt.Fprintf(c, saveChanges+commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// StartDegas runs a degas operation
func (c *RGAConnection) StartDegas(StartPower, EndPower, RampPeriod, MaxPowerPeriod, ResettlePeriod int) (*RGAResponse, error) {
	fmt.Fprintf(c, "%s %d %d %d %d %d%s", startDegas, StartPower, EndPower, RampPeriod, MaxPowerPeriod, ResettlePeriod, commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}

// StopDegas ends a degas operation that is currently running
func (c *RGAConnection) StopDegas() (*RGAResponse, error) {
	fmt.Fprintf(c, stopDegas+commandSuffix)
	buf := make([]byte, BUFFER)
	_, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := bytes.Split(buf, commandEnd) // The whole response minus the empty bytes leftover
	return parseVerticalResp(resp[0], false)
}
