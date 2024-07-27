package device

import (
	"fmt"
	"time"

	hid "github.com/sstallion/go-hid"
)

const (
	readDeviceTimeout   = 30 * time.Millisecond
	readPacketFrequency = 10 * time.Millisecond

	waitForPacketTimeout = 1 * time.Second
	retryMaxAttempts     = 3

	heartBeatTimeout = 500 * time.Millisecond
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

	GetImages(folderpath string) ([]string, error)

	EnableEventReporting(event CommandInstruction, enabled string) error

	SetAmbientLightEventHandler(handler AmbientLightEventHandler)
	SetKeyEventHandler(handler KeyEventHandler)
	SetMagnetometerEventHandler(handler MagnetometerEventHandler)
	SetProximityEventHandler(handler ProximityEventHandler)
	SetTemperatureEventHandler(handler TemperatureEventHandlder)
	SetVSyncEventHandler(handler VSyncEventHandler)

	// For development testing only
	DevExecuteAndRead(device string, intput []string)
	GetImagesDataDev(folderpath string) ([]string, error)
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
	AmbientLightEventHandler AmbientLightEventHandler
	KeyEventHandler          KeyEventHandler
	MagnetometerEventHandler MagnetometerEventHandler
	ProximityEventHandler    ProximityEventHandler
	TemperatureEventHandlder TemperatureEventHandlder
	VSyncEventHandler        VSyncEventHandler
	IMUEventHandler          IMUEventHandler
}

type AmbientLightEventHandler func(uint16)
type VSyncEventHandler func(string)
type TemperatureEventHandlder func(string)

type MagnetometerEventHandler func(*MagnetometerVector)

type MagnetometerVector struct {
	// TODO(happyz): Parse X,Y,Z
	X         int
	Y         int
	Z         int
	Timestamp time.Time
}

func (mv MagnetometerVector) String() string {
	return fmt.Sprintf("(x,y,z)=(%d, %d, %d) at %v", mv.X, mv.Y, mv.Z, mv.Timestamp)
}

type KeyEventHandler func(KeyEvent)
type KeyEvent uint8

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
type ProximityEvent uint8

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

type IMUEventHandler func(*IMUEvent)
type IMUEvent struct {
	Accelerometer *AccelerometerVector
	Gyroscope     *GyroscopeVector
	// TimeSinceBoot is in miliseconds
	TimeSinceBoot uint64
}

func (imu IMUEvent) String() string {
	return fmt.Sprintf("accel: %s, gyro: %s, at %d ms since boot", imu.Accelerometer.String(), imu.Gyroscope.String(), imu.TimeSinceBoot)
}

type AccelerometerVector struct {
	X float32
	Y float32
	Z float32
}

func (accel AccelerometerVector) String() string {
	return fmt.Sprintf("(x,y,z)=(%f, %f, %f)", accel.X, accel.Y, accel.Z)
}

type GyroscopeVector struct {
	X float32
	Y float32
	Z float32
}

func (gyro GyroscopeVector) String() string {
	return fmt.Sprintf("(x,y,z)=(%f, %f, %f)", gyro.X, gyro.Y, gyro.Z)
}

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

func getTimestampNow() []byte {
	return []byte(fmt.Sprintf("%x", (time.Now().UnixMilli())))
}

// TODO(happyz): Adds hotplug detection once https://github.com/libusb/hidapi/pull/674 is resolved.
