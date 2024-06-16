package device

import (
	"fmt"
	"log/slog"
	"time"

	"xreal-light-xr-go/constant"
)

const (
	readDeviceTimeout   = 30 * time.Millisecond
	readPacketFrequency = 10 * time.Millisecond

	waitForPacketTimeout = 1 * time.Second
	retryMaxAttempts     = 3

	heartBeatTimeout = 500 * time.Millisecond
)

type xrealLight struct {
	mcu   *xrealLightMCU
	ov580 *xrealLightOV580
}

func (l *xrealLight) Name() string {
	return constant.XREAL_LIGHT
}

func (l *xrealLight) PID() uint16 {
	return XREAL_LIGHT_MCU_PID
}

func (l *xrealLight) VID() uint16 {
	return XREAL_LIGHT_MCU_VID
}

func (l *xrealLight) Disconnect() error {
	errMCU := l.mcu.disconnect()
	errOV580 := l.ov580.disconnect()

	if errMCU != nil || errOV580 != nil {
		return fmt.Errorf("mcu err: %w; 0v580 err: %w", errMCU, errOV580)
	}
	return nil
}

func (l *xrealLight) Connect() error {
	errMCU := l.mcu.connectAndInitialize()
	errOV580 := l.ov580.connectAndInitialize()

	if errMCU != nil || errOV580 != nil {
		l.Disconnect()
		return fmt.Errorf("mcu err: %w; 0v580 err: %w", errMCU, errOV580)
	}
	return nil
}

func (l *xrealLight) GetSerial() (string, error) {
	return l.mcu.getSerial()
}

func (l *xrealLight) GetFirmwareVersion() (string, error) {
	if l.mcu.device == nil {
		return "", fmt.Errorf("glass device is not connected yet")
	}
	return l.mcu.glassFirmware, nil
}

func (l *xrealLight) GetDisplayMode() (DisplayMode, error) {
	return l.mcu.getDisplayMode()
}

func (l *xrealLight) SetDisplayMode(mode DisplayMode) error {
	return l.mcu.setDisplayMode(mode)
}

func (l *xrealLight) GetBrightnessLevel() (string, error) {
	return l.mcu.getBrightnessLevel()
}

func (l *xrealLight) SetBrightnessLevel(level string) error {
	return l.mcu.setBrightnessLevel(level)
}

func (l *xrealLight) EnableEventReporting(instruction CommandInstruction, enabled string) error {
	switch instruction {
	case OV580_ENABLE_IMU_STREAM:
		return l.ov580.enableEventReporting(instruction, enabled)
	default:
		return l.mcu.enableEventReporting(instruction, enabled)
	}
}

func (l *xrealLight) SetAmbientLightEventHandler(handler AmbientLightEventHandler) {
	l.mcu.deviceHandlers.AmbientLightEventHandler = handler
}

func (l *xrealLight) SetKeyEventHandler(handler KeyEventHandler) {
	l.mcu.deviceHandlers.KeyEventHandler = handler
}

func (l *xrealLight) SetMagnetometerEventHandler(handler MagnetometerEventHandler) {
	l.mcu.deviceHandlers.MagnetometerEventHandler = handler
}

func (l *xrealLight) SetProximityEventHandler(handler ProximityEventHandler) {
	l.mcu.deviceHandlers.ProximityEventHandler = handler
}

func (l *xrealLight) SetTemperatureEventHandler(handler TemperatureEventHandlder) {
	l.mcu.deviceHandlers.TemperatureEventHandlder = handler
}

func (l *xrealLight) SetVSyncEventHandler(handler VSyncEventHandler) {
	l.mcu.deviceHandlers.VSyncEventHandler = handler
}

func (l *xrealLight) DevExecuteAndRead(input []string) {
	l.mcu.devExecuteAndRead(input)
}

func NewXREALLight(mcuDevicePath *string, ov580DevicePath *string) Device {
	var l xrealLight

	l.mcu = &xrealLightMCU{
		devicePath: mcuDevicePath,
		deviceHandlers: &DeviceHandlers{
			AmbientLightEventHandler: func(value uint16) {
				slog.Info(fmt.Sprintf("Ambient light: %d", value))
			},
			KeyEventHandler: func(key KeyEvent) {
				slog.Info(fmt.Sprintf("Key pressed: %s", key.String()))
			},
			MagnetometerEventHandler: func(vector *MagnetometerVector) {
				slog.Info(fmt.Sprintf("Magnetometer: %s", vector.String()))
			},
			ProximityEventHandler: func(proximity ProximityEvent) {
				slog.Info(fmt.Sprintf("Proximity: %s", proximity.String()))
			},
			TemperatureEventHandlder: func(value string) {
				slog.Info(fmt.Sprintf("Temperature: %s", value))
			},
			VSyncEventHandler: func(value string) {
				slog.Info(fmt.Sprintf("VSync: %s", value))
			},
		},
		packetResponseChannel:  make(chan *Packet),
		stopHeartBeatChannel:   make(chan struct{}),
		stopReadPacketsChannel: make(chan struct{}),
	}

	l.ov580 = &xrealLightOV580{
		devicePath:             ov580DevicePath,
		commandResponseChannel: make(chan []byte),
		stopReadDataChannel:    make(chan struct{}),
	}

	return &l
}
