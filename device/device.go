package device

import (
	hid "github.com/sstallion/go-hid"
)

// Device is an interface representing XREAL glasses.
type Device interface {
	Name() string
	PID() uint16
	VID() uint16

	Connect() error
	Disconnect() error

	GetSerial() (string, error)
	GetFirmwareVersion() (string, error)

	GetDisplayMode() (DisplayMode, error)
	SetDisplayMode(mode DisplayMode) error

	// For development testing only
	PrintCommandIDTable()
	DevExecuteAndRead(intput []string)
}

// DisplayMode represents the display mode of AR glasses.
type DisplayMode string

const (
	DISPLAY_MODE_UNKNOWN DisplayMode = "UNKNOWN"
	// SAME_ON_BOTH indicates that the picture should be the same for both eyes (simple 2D 1080p).
	DISPLAY_MODE_SAME_ON_BOTH DisplayMode = "SAME_ON_BOTH"
	// HALF_SBS sets the display to half-SBS mode, which presents itself as 1920x1080 resolution,
	// but actually scales down everything to 960x540, then upscales to 3840x1080.
	DISPLAY_MODE_HALF_SBS DisplayMode = "HALF_SBS"
	// STEREO sets the display to 1080p on both eyes.
	DISPLAY_MODE_STEREO DisplayMode = "STEREO"
	// HIGH_REFRESH_RATE sets the display at 1080p 72Hz high refresh rate mode.
	DISPLAY_MODE_HIGH_REFRESH_RATE DisplayMode = "HIGH_REFRESH_RATE"
)

var SupportedDisplayMode = map[string]struct{}{
	string(DISPLAY_MODE_SAME_ON_BOTH):      {},
	string(DISPLAY_MODE_HALF_SBS):          {},
	string(DISPLAY_MODE_STEREO):            {},
	string(DISPLAY_MODE_HIGH_REFRESH_RATE): {},
}

func enumerateDevices(vid, pid uint16) ([]*hid.DeviceInfo, error) {
	var devices []*hid.DeviceInfo
	err := hid.Enumerate(vid, pid, func(info *hid.DeviceInfo) error {
		devices = append(devices, info)
		return nil
	})
	return devices, err
}

// TODO(happyz): Adds hotplug detection once https://github.com/libusb/hidapi/pull/674 is resolved.
