package device

import (
	"fmt"

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

	GetDisplayMode() (DisplayMode, error)
	SetDisplayMode(mode DisplayMode) error
}

// DisplayMode represents the display mode of AR glasses.
type DisplayMode int

const (
	DISPLAY_MODE_UNKNOWN DisplayMode = iota
	// SAME_ON_BOTH indicates that the picture should be the same for both eyes (simple full HD mode).
	DISPLAY_MODE_SAME_ON_BOTH
	// HALF_SBS sets the display to half-SBS mode, which presents itself as 1920x1080 resolution,
	// but actually scales down everything to 960x540, then upscales to 3840x1080.
	DISPLAY_MODE_HALF_SBS
	// STEREO sets the display to 3840x1080 or 3840x1200, where the left half is the left eye and the right half is the right eye.
	DISPLAY_MODE_STEREO
	// HIGH_REFRESH_RATE sets the display to mirrored high refresh rate mode.
	DISPLAY_MODE_HIGH_REFRESH_RATE
)

func (dm DisplayMode) String() string {
	switch dm {
	case DISPLAY_MODE_UNKNOWN:
		return "Unknown"
	case DISPLAY_MODE_SAME_ON_BOTH:
		return "Same on Both"
	case DISPLAY_MODE_HALF_SBS:
		return "Half SBS"
	case DISPLAY_MODE_STEREO:
		return "Stereo"
	case DISPLAY_MODE_HIGH_REFRESH_RATE:
		return "High Refresh Rate"
	default:
		return fmt.Sprintf("Unknown Display Mode (%d)", dm)
	}
}

func enumerateDevices(vid, pid uint16) ([]*hid.DeviceInfo, error) {
	var devices []*hid.DeviceInfo
	err := hid.Enumerate(vid, pid, func(info *hid.DeviceInfo) error {
		devices = append(devices, info)
		return nil
	})
	return devices, err
}
