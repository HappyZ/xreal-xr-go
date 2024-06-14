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

	GetBrightnessLevel() (string, error)
	SetBrightnessLevel(level string) error

	GetDisplayMode() (DisplayMode, error)
	SetDisplayMode(mode DisplayMode) error

	GetOptionsEnabled(options []string) []string

	SetKeyEventHandler(handler KeyEventHandler)
	SetProximityEventHandler(handler ProximityEventHandler)

	// For development testing only
	DevExecuteAndRead(intput []string)
}

// DisplayMode represents the display mode of AR glasses.
type DisplayMode string

const (
	DISPLAY_MODE_UNKNOWN DisplayMode = "UNKNOWN"
	// SAME_ON_BOTH indicates that the picture should be the same for both eyes (simple 2D 1080p at 60Hz).
	DISPLAY_MODE_SAME_ON_BOTH DisplayMode = "SAME_ON_BOTH"
	// HALF_SBS sets the display to half-SBS mode, which presents itself as 1920x1080 resolution,
	// but actually scales down everything to 960x540 at 120Hz, then upscales to 3840x1080.
	DISPLAY_MODE_HALF_SBS DisplayMode = "HALF_SBS"
	// STEREO sets the display to 1080p on both eyes at 60Hz.
	DISPLAY_MODE_STEREO DisplayMode = "STEREO"
	// HIGH_REFRESH_RATE sets the display at 1080p at 72Hz high refresh rate mode.
	DISPLAY_MODE_HIGH_REFRESH_RATE DisplayMode = "HIGH_REFRESH_RATE"
)

type DeviceHandlers struct {
	KeyEventHandler       KeyEventHandler
	ProximityEventHandler ProximityEventHandler
}

type KeyEventHandler func(KeyEvent)
type KeyEvent int

func (e KeyEvent) String() string {
	switch e {
	case KEY_UP_PRESSED:
		return "UP"
	case KEY_DOWN_PRESSED:
		return "DOWN"
	default:
		return "UNKNOWN"
	}
}

const (
	KEY_UNKNOWN KeyEvent = iota
	KEY_UP_PRESSED
	KEY_DOWN_PRESSED
)

type ProximityEventHandler func(ProximityEvent)
type ProximityEvent int

func (e ProximityEvent) String() string {
	switch e {
	case PROXIMITY_NEAR:
		return "NEAR"
	case PROXIMITY_FAR:
		return "FAR"
	default:
		return "UNKNOWN"
	}
}

const (
	PROXIMITY_UKNOWN ProximityEvent = iota
	PROXIMITY_NEAR
	PROXIMITY_FAR
)

var SupportedDisplayMode = map[string]struct{}{
	string(DISPLAY_MODE_SAME_ON_BOTH):      {},
	string(DISPLAY_MODE_HALF_SBS):          {},
	string(DISPLAY_MODE_STEREO):            {},
	string(DISPLAY_MODE_HIGH_REFRESH_RATE): {},
}

func EnumerateDevices(vid, pid uint16) ([]*hid.DeviceInfo, error) {
	var devices []*hid.DeviceInfo
	uniquePaths := make(map[string]struct{})
	err := hid.Enumerate(vid, pid, func(info *hid.DeviceInfo) error {
		if _, ok := uniquePaths[info.Path]; !ok {
			uniquePaths[info.Path] = struct{}{}
			devices = append(devices, info)
		}
		return nil
	})
	return devices, err
}

// TODO(happyz): Adds hotplug detection once https://github.com/libusb/hidapi/pull/674 is resolved.
