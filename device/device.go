package device

import (
	hid "github.com/sstallion/go-hid"
)

// Device is an interface representing XREAL glasses.
type Device interface {
	Name() string
	GetDisplayMode() (DisplayMode, error)
	SetDisplayMode(mode DisplayMode) error
}

// DisplayMode represents the display mode of AR glasses.
type DisplayMode int

const (
	DISPLAY_MODE_UNKNOWN DisplayMode = iota
	// SAME_ON_BOTH indicates that the picture should be the same for both eyes (simple full HD mode).
	DISPLAY_MODE_SAME_ON_BOTH
	// STEREO sets the display to 3840x1080 or 3840x1200, where the left half is the left eye and the right half is the right eye.
	DISPLAY_MODE_STEREO
	// HALF_SBS sets the display to half-SBS mode, which presents itself as 1920x1080 resolution,
	// but actually scales down everything to 960x540, then upscales to 3840x1080.
	DISPLAY_MODE_HALF_SBS
	// HIGH_REFRESH_RATE sets the display to mirrored high refresh rate mode.
	DISPLAY_MODE_HIGH_REFRESH_RATE
)

func enumerateDevices(vid, pid uint16) ([]*hid.DeviceInfo, error) {
	var devices []*hid.DeviceInfo
	err := hid.Enumerate(vid, pid, func(info *hid.DeviceInfo) error {
		devices = append(devices, info)
		return nil
	})
	return devices, err
}
