package device

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

	hid "github.com/sstallion/go-hid"
)

const (
	// XREAL Light SLAM Camera and IMU
	XREAL_LIGHT_OV580_VID = uint16(0x05a9)
	XREAL_LIGHT_OV580_PID = uint16(0x0680)
)

type xrealLightOV580 struct {
	initialized bool

	device *hid.Device
	// serialNumber is optional and can be nil if not provided
	serialNumber *string
	// devicePath is optional and can be nil if not provided
	devicePath *string

	// deviceHandlers contains callback funcs for the events from the glass device
	deviceHandlers *DeviceHandlers

	// bias values for accelerometer and gyro
	accelerometerBias *AccelerometerVector
	gyroscopeBias     *GyroscopeVector

	// mutex for thread safety
	mutex sync.Mutex
	// channel to signal a command gets a response
	commandResponseChannel chan []byte
	// waitgroup to wait for multiple goroutines to stop
	waitgroup sync.WaitGroup
	// channel to signal data reading to stop
	stopReadDataChannel chan struct{}
}

func (l *xrealLightOV580) connectAndInitialize() error {
	devices, err := EnumerateDevices(XREAL_LIGHT_OV580_VID, XREAL_LIGHT_OV580_PID)
	if err != nil {
		return fmt.Errorf("failed to enumerate OV580 hid devices: %w", err)
	}

	if len(devices) == 0 {
		return fmt.Errorf("no XREAL Light glasses found: %v", devices)
	}

	if len(devices) > 1 && l.devicePath == nil && l.serialNumber == nil {
		var message = string("multiple XREAL Light glasses found, please specify either devicePath or serialNumber:\n")
		for _, info := range devices {
			message += "- path: " + info.Path + "\n" + "  serialNumber: " + info.SerialNbr + "\n"
		}
		return fmt.Errorf(message)
	}

	if l.devicePath != nil {
		if device, err := hid.OpenPath(*l.devicePath); err != nil {
			return fmt.Errorf("failed to open the device path %s: %w", *l.devicePath, err)
		} else {
			l.device = device
		}
	} else if l.serialNumber != nil {
		if device, err := hid.Open(XREAL_LIGHT_OV580_VID, XREAL_LIGHT_OV580_PID, *l.serialNumber); err != nil {
			return fmt.Errorf("failed to open the device with serial number %s: %w", *l.serialNumber, err)
		} else {
			l.device = device
		}
	} else {
		if device, err := hid.OpenFirst(XREAL_LIGHT_OV580_VID, XREAL_LIGHT_OV580_PID); err != nil {
			return fmt.Errorf("failed to open the first hid device for XREAL Light OV580: %w", err)
		} else {
			l.device = device
		}
	}

	// backfill missing data
	if info, err := l.device.GetDeviceInfo(); err == nil {
		l.devicePath = &info.Path
		l.serialNumber = &info.SerialNbr
	}

	return l.initialize()
}

func (l *xrealLightOV580) initialize() error {
	l.waitgroup.Add(1)
	go l.readPacketsPeriodically()

	// ensure we get calibration file
	for {
		if err := l.readAndParseCalibrationConfigs(); err == nil {
			break
		} else {
			slog.Error(fmt.Sprintf("readAndParseCalibrationConfigs() failed, retrying: %v", err))
		}
	}

	l.initialized = true
	return nil
}

func (l *xrealLightOV580) readAndParseCalibrationConfigs() error {
	// disable IMU stream first to reduce noise
	if err := l.enableEventReporting(OV580_ENABLE_IMU_STREAM, "0"); err != nil {
		return err
	}

	command := GetFirmwareIndependentCommand(OV580_GET_CALIBRATION_FILE_LENGTH)
	response, err := l.executeAndWaitForResponse(command, 0x1)
	if err != nil {
		return fmt.Errorf("failed to %s: %w", command.String(), err)
	}
	fileLength := response[3:6]
	slog.Debug(fmt.Sprintf("calibration file length: %v", fileLength))

	command = GetFirmwareIndependentCommand(OV580_GET_CALIBRATION_FILE_PART)
	fileBytes := []byte{}
	for {
		response, err := l.executeAndWaitForResponse(command, 0x1)
		if err != nil {
			return fmt.Errorf("failed to %s: %w", command.String(), err)
		}
		if response[1] == 0x3 {
			break
		}
		fileBytes = append(fileBytes, response[3:(3+response[2])]...)
	}

	// enable IMU stream
	if err := l.enableEventReporting(OV580_ENABLE_IMU_STREAM, "1"); err != nil {
		return err
	}

	return l.parseCalibrationConfigs(fileBytes)
}

func (l *xrealLightOV580) parseCalibrationConfigs(fileBytes []byte) error {
	content := string(fileBytes)

	startIdx := strings.Index(content, "<")
	endIdx := strings.LastIndex(content, ">")
	xmlString := content[startIdx:(endIdx + 1)]
	slog.Debug(fmt.Sprintf("xml content: %s", xmlString))

	startIdx = strings.Index(content, "{")
	endIdx = strings.LastIndex(content, "}")
	jsonBytes := fileBytes[startIdx:(endIdx + 1)]
	var jsonData map[string]interface{}
	err := json.Unmarshal(jsonBytes, &jsonData)
	if err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}
	slog.Debug(fmt.Sprintf("json content: %s", jsonData))

	device1Data := jsonData["IMU"].(map[string]interface{})["device_1"].(map[string]interface{})

	accelBias := device1Data["accel_bias"].([]interface{})
	l.accelerometerBias = &AccelerometerVector{
		X: float32(accelBias[0].(float64)),
		Y: float32(accelBias[1].(float64)),
		Z: float32(accelBias[2].(float64)),
	}

	gyroBias := device1Data["gyro_bias"].([]interface{})
	l.gyroscopeBias = &GyroscopeVector{
		X: float32(gyroBias[0].(float64)),
		Y: float32(gyroBias[1].(float64)),
		Z: float32(gyroBias[2].(float64)),
	}

	slog.Debug(fmt.Sprintf("remaining content: %s", content[(endIdx+1):]))

	return nil
}

// readPacketsPeriodically is a goroutine method to read info from XREAL Light MCU HID device
func (l *xrealLightOV580) readPacketsPeriodically() {
	defer l.waitgroup.Done()

	ticker := time.NewTicker(readPacketFrequency)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := l.readAndProcessData(); err != nil {
				if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "timed out") || strings.Contains(err.Error(), "system call") {
					continue
				}
				slog.Debug(fmt.Sprintf("readAndProcessData(): %v", err))
			}
		case <-l.stopReadDataChannel:
			return
		}
	}
}

func (l *xrealLightOV580) executeAndWaitForResponse(command *Command, value uint8) ([]byte, error) {
	if err := l.executeOnly(command, value); err != nil {
		return nil, err
	}
	for retry := 0; retry < retryMaxAttempts; retry++ {
		select {
		case response := <-l.commandResponseChannel:
			return response, nil
		case <-time.After(waitForPacketTimeout):
			if retry < retryMaxAttempts-1 {
				continue
			}
			return nil, fmt.Errorf("failed to get response for %s: timed out", command.String())
		}
	}

	return nil, fmt.Errorf("failed to get a relevant response for %s: exceed max retries (%d)", command.String(), retryMaxAttempts)
}

func (l *xrealLightOV580) executeOnly(command *Command, value uint8) error {
	l.mutex.Lock()

	defer l.mutex.Unlock()

	if l.device == nil {
		return fmt.Errorf("not connected / initialized")
	}

	_, err := l.device.Write([]byte{command.Type, command.ID, value, 0, 0, 0, 0})
	if err != nil {
		return fmt.Errorf("failed to execute on device %v: %w", l.device, err)
	}
	return nil
}

// readAndProcessData receives data piece from OV580 device to be processed.
// This method should be called as frequently as possible to track the time of the packets more accurately.
func (l *xrealLightOV580) readAndProcessData() error {
	var buffer [128]byte
	_, err := l.device.ReadWithTimeout(buffer[:], readDeviceTimeout)
	if err != nil {
		return fmt.Errorf("failed to read from device %v: %w", l.device, err)
	}

	switch buffer[0] {
	case 0x1: // IMU event
		// don't do anything if not yet initialized
		if !l.initialized {
			return nil
		}

		reader := bytes.NewReader(buffer[0x2a:])

		var temperature uint16
		binary.Read(reader, binary.LittleEndian, &temperature)
		slog.Debug(fmt.Sprintf("temperature: %d", temperature))

		var gyroTimestamp uint64 // nanoseconds
		binary.Read(reader, binary.LittleEndian, &gyroTimestamp)

		var gyroMultiplierRaw uint32
		binary.Read(reader, binary.LittleEndian, &gyroMultiplierRaw)
		gyroMultiplier := float32(gyroMultiplierRaw)

		var gyroDivisorRaw uint32
		binary.Read(reader, binary.LittleEndian, &gyroDivisorRaw)
		gyroDivisor := float32(gyroDivisorRaw)

		var gyroXRaw int32
		binary.Read(reader, binary.LittleEndian, &gyroXRaw)
		gyroX := float32(gyroXRaw)

		var gyroYRaw int32
		binary.Read(reader, binary.LittleEndian, &gyroYRaw)
		gyroY := float32(gyroYRaw)

		var gyroZRaw int32
		binary.Read(reader, binary.LittleEndian, &gyroZRaw)
		gyroZ := float32(gyroZRaw)

		gyro := &GyroscopeVector{
			X: (gyroX*gyroMultiplier/gyroDivisor)*(math.Pi/180.0) - l.gyroscopeBias.X,
			Y: -(gyroY*gyroMultiplier/gyroDivisor)*(math.Pi/180.0) + l.gyroscopeBias.Y,
			Z: -(gyroZ*gyroMultiplier/gyroDivisor)*(math.Pi/180.0) + l.gyroscopeBias.Z,
		}

		var accelTimestamp uint64 // nanoseconds
		binary.Read(reader, binary.LittleEndian, &accelTimestamp)

		var accelMultiplierRaw uint32
		binary.Read(reader, binary.LittleEndian, &accelMultiplierRaw)
		accelMultiplier := float32(accelMultiplierRaw)

		var accelDivisorRaw uint32
		binary.Read(reader, binary.LittleEndian, &accelDivisorRaw)
		accelDivisor := float32(accelDivisorRaw)

		var accelXRaw int32
		binary.Read(reader, binary.LittleEndian, &accelXRaw)
		accelX := float32(accelXRaw)

		var accelYRaw int32
		binary.Read(reader, binary.LittleEndian, &accelYRaw)
		accelY := float32(accelYRaw)

		var accelZRaw int32
		binary.Read(reader, binary.LittleEndian, &accelZRaw)
		accelZ := float32(accelZRaw)

		accel := &AccelerometerVector{
			X: (accelX*accelMultiplier/accelDivisor)*9.81 - l.accelerometerBias.X,
			Y: -(accelY*accelMultiplier/accelDivisor)*9.81 + l.accelerometerBias.Y,
			Z: -(accelZ*accelMultiplier/accelDivisor)*9.81 + l.accelerometerBias.Z,
		}

		if gyroTimestamp != accelTimestamp {
			slog.Warn(fmt.Sprintf("odd, found gyro and accel with different timestamp: %d vs %d nanoseconds", gyroTimestamp, accelTimestamp))
		}

		imu := &IMUEvent{
			Gyroscope:     gyro,
			Accelerometer: accel,
			TimeSinceBoot: gyroTimestamp / 1000000, // miliseconds
		}
		l.deviceHandlers.IMUEventHandler(imu)
		return nil
	case 0x2:
		switch buffer[1] {
		case 0x0: // calibration file length
			l.commandResponseChannel <- buffer[:]
			return nil
		case 0x4: // acknowleging IMU enabled
			l.commandResponseChannel <- buffer[:]
			return nil
		case 0x1: // reading calibration file continue
			l.commandResponseChannel <- buffer[:]
			return nil
		case 0x3: // ending calibration file read
			l.commandResponseChannel <- buffer[:]
			return nil
		default:
			l.commandResponseChannel <- buffer[:]
			slog.Debug(fmt.Sprintf("buffer[1] = %d", buffer[1]))
			return nil
		}
	default:
	}

	slog.Debug(fmt.Sprintf("got unhandled readings: %v", buffer[:]))

	return nil
}

func (l *xrealLightOV580) enableEventReporting(instruction CommandInstruction, enabled string) error {
	command := GetFirmwareIndependentCommand(instruction)
	value := uint8(0x0)
	if enabled == "1" {
		value = 0x1
	}
	for retry := 0; retry < retryMaxAttempts; retry++ {
		if response, err := l.executeAndWaitForResponse(command, value); err == nil {
			if (response[0] != 0x2) && (response[0] != 0x4) {
				return fmt.Errorf("failed to set event reporting: want [0x2 0x4] got %v", response)
			}
			return nil
		}
	}
	return fmt.Errorf("failed to set event reporting: exceed max attempts to execute")
}

func (l *xrealLightOV580) devExecuteAndRead(input []string) {
	if len(input) != 3 {
		slog.Error(fmt.Sprintf("wrong input format: want hex string for [CommandType CommandID Payload] got %v", input))
		return
	}

	commandType, err := hexStringToBytes(input[0])
	if err != nil {
		slog.Error(err.Error())
	}
	commandID, err := hexStringToBytes(input[1])
	if err != nil {
		slog.Error(err.Error())
	}
	value, err := hexStringToBytes(input[2])
	if err != nil {
		slog.Error(err.Error())
	}

	command := &Command{Type: commandType[0], ID: commandID[0]}
	response, err := l.executeAndWaitForResponse(command, value[0])
	if err != nil {
		slog.Error(fmt.Sprintf("%s : '%v' failed: %v", command.String(), response, err))
		return
	}
	slog.Info(fmt.Sprintf("%s : '%v'", command.String(), response))
}

func hexStringToBytes(hexString string) ([]byte, error) {
	if len(hexString)%2 != 0 {
		hexString = "0" + hexString // Pad with '0' at the beginning
	}

	if byteArray, err := hex.DecodeString(hexString); err == nil {
		return byteArray, nil
	} else {
		return nil, fmt.Errorf("failed to convert %s to hex: %w", hexString, err)
	}
}

func (l *xrealLightOV580) disconnect() error {
	l.initialized = false

	if l.device == nil {
		return nil
	}

	close(l.stopReadDataChannel)

	l.waitgroup.Wait()

	close(l.commandResponseChannel)

	err := l.device.Close()
	if err == nil {
		l.device = nil
	}
	return err
}
