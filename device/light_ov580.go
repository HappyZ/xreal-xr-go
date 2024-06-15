package device

import (
	"fmt"

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
		if device, err := hid.Open(XREAL_LIGHT_MCU_VID, XREAL_LIGHT_MCU_PID, *l.serialNumber); err != nil {
			return fmt.Errorf("failed to open the device with serial number %s: %w", *l.serialNumber, err)
		} else {
			ov580Device = device
		}
	} else {
		if device, err := hid.OpenFirst(XREAL_LIGHT_MCU_VID, XREAL_LIGHT_MCU_PID); err != nil {
			return fmt.Errorf("failed to open the first hid device for XREAL Light OV580: %w", err)
		} else {
			ov580Device = device
		}
	}

	l.device = ov580Device
	return l.initialize()
}

func (l *xrealLightOV580) initialize() error {
	return nil
}

func (l *xrealLightOV580) disconnect() error {
	if l.device == nil {
		return nil
	}

	err := l.device.Close()
	if err == nil {
		l.device = nil
	}
	return err
}
