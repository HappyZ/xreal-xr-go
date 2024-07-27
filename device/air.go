package device

import (
	"fmt"
	"log/slog"

	"xreal-light-xr-go/constant"
)

type AirModel int

const (
	AIR_MODEL_UNKNOWN AirModel = iota
	AIR_MODEL_AIR
	AIR_MODEL_AIR_2
	AIR_MODEL_AIR_2_PRO
	AIR_MODEL_AIR_2_ULTRA
)

func (model AirModel) String() string {
	switch model {
	case AIR_MODEL_AIR:
		return constant.XREAL_AIR
	case AIR_MODEL_AIR_2:
		return constant.XREAL_AIR_2
	case AIR_MODEL_AIR_2_PRO:
		return constant.XREAL_AIR_2_PRO
	case AIR_MODEL_AIR_2_ULTRA:
		return constant.XREAL_AIR_2_ULTRA
	default:
		return "unknown"
	}
}

func (model AirModel) PID() uint16 {
	switch model {
	case AIR_MODEL_AIR:
		return XREAL_AIR_MCU_PID
	case AIR_MODEL_AIR_2:
		return XREAL_AIR_2_MCU_PID
	case AIR_MODEL_AIR_2_PRO:
		return XREAL_AIR_2_PRO_MCU_PID
	default:
		return 0
	}
}

type xrealAir struct {
	model AirModel
	mcu   *xrealAirMCU
}

func (a *xrealAir) Name() string {
	return a.model.String()
}

func (a *xrealAir) PID() uint16 {
	return a.model.PID()
}

func (a *xrealAir) VID() uint16 {
	return XREAL_AIR_SERIES_MCU_VID
}

func (a *xrealAir) Disconnect() error {
	return fmt.Errorf("unimplemneted")
	// errMCU := a.mcu.disconnect()

	// if errMCU != nil {
	// 	return errMCU
	// }
	// return nil
}

func (a *xrealAir) Connect() error {
	errMCU := a.mcu.connectAndInitialize(a.VID(), a.PID())

	if errMCU != nil {
		a.Disconnect()
		return errMCU
	}
	return nil
}

func (a *xrealAir) GetSerial() (string, error) {
	return "", fmt.Errorf("unimplemneted")
	// return a.mcu.getSerial()
}

func (a *xrealAir) GetFirmwareVersion() (string, error) {
	return "", fmt.Errorf("unimplemneted")
	// if a.mcu.device == nil {
	// 	return "", fmt.Errorf("glass device is not connected yet")
	// }
	// return a.mcu.glassFirmware, nil
}

func (a *xrealAir) GetDisplayMode() (DisplayMode, error) {
	return DISPLAY_MODE_UNKNOWN, fmt.Errorf("unimplemneted")
	// return a.mcu.getDisplayMode()
}

func (a *xrealAir) SetDisplayMode(mode DisplayMode) error {
	return fmt.Errorf("unimplemneted")
	// return a.mcu.setDisplayMode(mode)
}

func (a *xrealAir) GetBrightnessLevel() (string, error) {
	return "", fmt.Errorf("unimplemneted")
	// return a.mcu.getBrightnessLevel()
}

func (a *xrealAir) SetBrightnessLevel(level string) error {
	return fmt.Errorf("unimplemneted")
	// return a.mcu.setBrightnessLevel(level)
}

func (a *xrealAir) EnableEventReporting(instruction CommandInstruction, enabled string) error {
	return fmt.Errorf("unimplemneted")
	// return a.mcu.enableEventReporting(instruction, enabled)
}

func (a *xrealAir) SetAmbientLightEventHandler(handler AmbientLightEventHandler) {
	a.mcu.deviceHandlers.AmbientLightEventHandler = handler
}

func (a *xrealAir) SetKeyEventHandler(handler KeyEventHandler) {
	a.mcu.deviceHandlers.KeyEventHandler = handler
}

func (a *xrealAir) SetMagnetometerEventHandler(handler MagnetometerEventHandler) {
	a.mcu.deviceHandlers.MagnetometerEventHandler = handler
}

func (a *xrealAir) SetProximityEventHandler(handler ProximityEventHandler) {
	a.mcu.deviceHandlers.ProximityEventHandler = handler
}

func (a *xrealAir) SetTemperatureEventHandler(handler TemperatureEventHandlder) {
	a.mcu.deviceHandlers.TemperatureEventHandlder = handler
}

func (a *xrealAir) SetVSyncEventHandler(handler VSyncEventHandler) {
	a.mcu.deviceHandlers.VSyncEventHandler = handler
}

func (a *xrealAir) DevExecuteAndRead(device string, input []string) {
	// if device == "mcu" {
	// 	a.mcu.devExecuteAndRead(input)
	// }
}

func (a *xrealAir) GetImages(folderpath string) ([]string, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (a *xrealAir) GetImagesDataDev(folderpath string) ([]string, error) {
	return nil, fmt.Errorf("unimplemented")
}

// NewXREALAir creates a xrealAir instance initiating MCU connections.
// TODO(happyz): Supports multiple glasses connected.
func NewXREALAir() Device {
	var a xrealAir

	a.mcu = &xrealAirMCU{
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

	return &a
}
