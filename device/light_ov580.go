package device

import (
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	hid "github.com/sstallion/go-hid"
)

const (
	// XREAL Light Camera and IMU
	XREAL_LIGHT_OV580_VID = uint16(0x05a9)
	XREAL_LIGHT_OV580_PID = uint16(0x0680)
)

type xrealLightOV580 struct {
	device *hid.Device
	// serialNumber is optional and can be nil if not provided
	serialNumber *string
	// devicePath is optional and can be nil if not provided
	devicePath *string

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

	var ov580Device *hid.Device

	if l.devicePath != nil {
		if device, err := hid.OpenPath(*l.devicePath); err != nil {
			return fmt.Errorf("failed to open the device path %s: %w", *l.devicePath, err)
		} else {
			ov580Device = device
		}
	} else if l.serialNumber != nil {
		if device, err := hid.Open(XREAL_LIGHT_OV580_VID, XREAL_LIGHT_OV580_PID, *l.serialNumber); err != nil {
			return fmt.Errorf("failed to open the device with serial number %s: %w", *l.serialNumber, err)
		} else {
			ov580Device = device
		}
	} else {
		if device, err := hid.OpenFirst(XREAL_LIGHT_OV580_VID, XREAL_LIGHT_OV580_PID); err != nil {
			return fmt.Errorf("failed to open the first hid device for XREAL Light OV580: %w", err)
		} else {
			ov580Device = device
		}
	}

	l.device = ov580Device
	return l.initialize()
}

func (l *xrealLightOV580) initialize() error {
	l.waitgroup.Add(1)
	go l.readPacketsPeriodically()
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
				slog.Debug(fmt.Sprintf("timed out waiting for response for %s, retry", command.String()))
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
	case 0x1:
		// TODO(happyz): Handles event data
		return nil
	case 0x2:
		switch buffer[1] {
		case 0x4:
			l.commandResponseChannel <- buffer[:]
			return nil
		default:
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
	if _, err := l.executeAndWaitForResponse(command, value); err != nil {
		return fmt.Errorf("failed to set event reporting: %w", err)
	}
	return nil
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
	l.executeAndWaitForResponse(command, value[0])
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
