package device

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"xreal-light-xr-go/constant"

	hid "github.com/sstallion/go-hid"
)

const (
	readDeviceTimeout   = 30 * time.Millisecond
	readPacketFrequency = 10 * time.Millisecond

	waitForPacketTimeout = 1 * time.Second
	retryMaxAttempts     = 3

	heartBeatTimeout = 500 * time.Millisecond
)

const (
	// XREAL Light MCU
	XREAL_LIGHT_MCU_VID = uint16(0x0486)
	XREAL_LIGHT_MCU_PID = uint16(0x573c)
	// XREAL Light Camera and IMU
	XREAL_LIGHT_OV580_VID = uint16(0x05a9)
	XREAL_LIGHT_OV580_PID = uint16(0x0680)
)

type xrealLight struct {
	mcuDevice *hid.Device
	// serialNumber is optional and can be nil if not provided
	serialNumber *string
	// devicePath is optional and can be nil if not provided
	devicePath *string

	// deviceHandlers contains callback funcs
	deviceHandlers DeviceHandlers

	// mutex for thread safety
	mutex sync.Mutex
	// waitgroup to wait for multiple goroutines to stop
	waitgroup sync.WaitGroup
	// channel to signal heart beat to stop
	stopHeartBeatChannel chan struct{}
	// channel to signal packet reading to stop
	stopReadPacketsChannel chan struct{}
	// channel to signal a command packet response
	packetResponseChannel chan *Packet
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
	if l.mcuDevice == nil {
		return nil
	}

	close(l.stopHeartBeatChannel)
	close(l.stopReadPacketsChannel)

	l.waitgroup.Wait()

	close(l.packetResponseChannel)

	err := l.mcuDevice.Close()
	if err == nil {
		l.mcuDevice = nil
	}
	return err
}

func (l *xrealLight) Connect() error {
	devices, err := EnumerateDevices(XREAL_LIGHT_MCU_VID, XREAL_LIGHT_MCU_PID)
	if err != nil {
		return fmt.Errorf("failed to enumerate hid devices: %w", err)
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

	var mcuDevice *hid.Device

	if l.devicePath != nil {
		if device, err := hid.OpenPath(*l.devicePath); err != nil {
			return fmt.Errorf("failed to open the device path %s: %w", *l.devicePath, err)
		} else {
			mcuDevice = device
		}
	} else if l.serialNumber != nil {
		if device, err := hid.Open(XREAL_LIGHT_MCU_VID, XREAL_LIGHT_MCU_PID, *l.serialNumber); err != nil {
			return fmt.Errorf("failed to open the device with serial number %s: %w", *l.serialNumber, err)
		} else {
			mcuDevice = device
		}
	} else {
		if device, err := hid.OpenFirst(XREAL_LIGHT_MCU_VID, XREAL_LIGHT_MCU_PID); err != nil {
			return fmt.Errorf("failed to open the first hid device for XREAL Light MCU: %w", err)
		} else {
			mcuDevice = device
		}
	}

	l.mcuDevice = mcuDevice

	return l.initialize()
}

func (l *xrealLight) initialize() error {
	l.waitgroup.Add(1)
	go l.sendHeartBeatPeriodically()

	l.waitgroup.Add(1)
	go l.readPacketsPeriodically()

	return nil
}

func (l *xrealLight) sendHeartBeatPeriodically() {
	defer l.waitgroup.Done()

	ticker := time.NewTicker(heartBeatTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := l.executeOnly(newCommandPacket(&CMD_HEART_BEAT)); err != nil {
				slog.Debug(fmt.Sprintf("failed to send a heartbeat: %v", err))
			}
		case <-l.stopHeartBeatChannel:
			return
		}
	}
}

// readPacketsPeriodically is a goroutine method to read info from XREAL Light HID device
func (l *xrealLight) readPacketsPeriodically() {
	defer l.waitgroup.Done()

	ticker := time.NewTicker(readPacketFrequency)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := l.readAndProcessPackets(); err != nil {
				slog.Debug(fmt.Sprintf("readAndProcessPackets(): %v", err))
			}
		case <-l.stopReadPacketsChannel:
			return
		}
	}
}

func (l *xrealLight) executeOnly(command *Packet) error {
	l.mutex.Lock()

	defer l.mutex.Unlock()

	if serialized, err := command.Serialize(); err != nil {
		return fmt.Errorf("failed to serialize command %v: %w", command, err)
	} else {
		if _, err := l.mcuDevice.Write(serialized[:]); err != nil {
			return fmt.Errorf("failed to execute on device %v: %w", l.mcuDevice, err)
		}
	}
	return nil
}

func read(device *hid.Device, timeout time.Duration) ([64]byte, error) {
	var buffer [64]byte

	_, err := device.ReadWithTimeout(buffer[:], timeout)
	if err != nil {
		return buffer, fmt.Errorf("failed to read from device %v: %w", device, err)
	}

	return buffer, nil
}

// readAndProcessPackets sends a legit packet request to device and receives a set of packets to be processed.
// This method should be called as frequently as possible to track the time of the packets more accurately.
func (l *xrealLight) readAndProcessPackets() error {
	commandPacket := newCommandPacket(&CMD_GET_NREAL_FW_STRING)
	// we must send a packet to get all responses, which is a bit lame
	if err := l.executeOnly(commandPacket); err != nil {
		return err
	}
	for i := 0; i < 32; i++ {
		buffer, err := read(l.mcuDevice, readDeviceTimeout)
		if err != nil {
			return err
		}

		response := &Packet{}

		if err := response.Deserialize(buffer[:]); err != nil {
			slog.Debug(fmt.Sprintf("failed to deserialize %v (%s): %v", buffer, string(buffer[:]), err))
			continue
		}

		if response.Type == PACKET_TYPE_CRC_ERROR || response.Type == PACKET_TYPE_HEART_BEAT_RESPONSE {
			// skip if CRC error packet or is a heart beat response
			continue
		}

		if (response.Command.Type == commandPacket.Command.Type+1) && (response.Command.ID == commandPacket.Command.ID) {
			// we ignore the legit response to our prior command as it's not useful for us
			// but we stop here
			return nil
		}

		// handle response by checking the Type, we assume only one execution happens at a time
		if response.Type == PACKET_TYPE_RESPONSE {
			l.packetResponseChannel <- response
			continue
		}

		// handle MCU
		if response.Type == PACKET_TYPE_MCU {
			if response.Command.Equals(&MCU_KEY_PRESS) {
				switch string(response.Payload) {
				case "UP":
					l.deviceHandlers.KeyEventHandler(KEY_UP_PRESSED)
				case "DN":
					l.deviceHandlers.KeyEventHandler(KEY_DOWN_PRESSED)
				default:
					slog.Debug(fmt.Sprintf("Key pressed unrecognized: %s", string(response.Payload)))
					l.deviceHandlers.KeyEventHandler(KEY_UNKNOWN)
				}
			} else if response.Command.Equals(&MCU_PROXIMITY) {
				switch string(response.Payload) {
				case "far":
					l.deviceHandlers.ProximityEventHandler(PROXIMITY_FAR)
				case "near":
					l.deviceHandlers.ProximityEventHandler(PROXIMITY_NEAR)
				default:
					slog.Info(fmt.Sprintf("Proximity unrecognized: %s", string(response.Payload)))
					l.deviceHandlers.ProximityEventHandler(PROXIMITY_UKNOWN)
				}
			} else if response.Command.Equals(&MCU_AMBIENT_LIGHT) {
				if value, err := strconv.ParseUint(string(response.Payload), 10, 16); err != nil {
					slog.Debug(fmt.Sprintf("Ambient light failed to parse: %s", string(response.Payload)))
				} else {
					l.deviceHandlers.AmbientLightEventHandler(uint16(value))
				}
			} else if response.Command.Equals(&MCU_VSYNC) {
				l.deviceHandlers.VSyncEventHandler(string(response.Payload))
			} else if response.Command.Equals(&MCU_MAGNETOMETER) {
				reading := string(response.Payload)

				xIdx := strings.Index(reading, "x")
				yIdx := strings.Index(reading, "y")
				zIdx := strings.Index(reading, "z")

				x, err := strconv.Atoi(reading[xIdx+1 : yIdx])
				if err != nil {
					slog.Debug(fmt.Sprintf("failed to parse %s to integer", reading[xIdx+1:yIdx]))
					continue
				}

				y, err := strconv.Atoi(reading[yIdx+1 : zIdx])
				if err != nil {
					slog.Debug(fmt.Sprintf("failed to parse %s to integer", reading[yIdx+1:zIdx]))
					continue
				}

				z, err := strconv.Atoi(reading[zIdx+1:])
				if err != nil {
					slog.Debug(fmt.Sprintf("failed to parse %s to integer", reading[zIdx+1:]))
					continue
				}

				l.deviceHandlers.MagnetometerEventHandler(
					&MagnetometerVector{
						X:         x,
						Y:         y,
						Z:         z,
						Timestamp: response.DecodeTimestamp(),
					},
				)
			} else {
				slog.Debug(fmt.Sprintf("got unhandled MCU packet: %v %s", response.Command, string(response.Payload)))
			}
			continue
		}

		slog.Debug(fmt.Sprintf("got unhandled packet: %v from %s", response, string(buffer[:])))
	}

	return nil
}

func (l *xrealLight) executeAndWaitForResponse(command *Packet) ([]byte, error) {
	if err := l.executeOnly(command); err != nil {
		return nil, err
	}
	for retry := 0; retry < retryMaxAttempts; retry++ {
		select {
		case response := <-l.packetResponseChannel:
			if (response.Command.Type == command.Command.Type+1) && (response.Command.ID == command.Command.ID) {
				return response.Payload, nil
			}
		case <-time.After(waitForPacketTimeout):
			if retry < retryMaxAttempts-1 {
				slog.Debug(fmt.Sprintf("timed out waiting for packet response for %s, retry", command.String()))
				continue
			}
			return nil, fmt.Errorf("failed to get response for %s: timed out", command.String())
		}
	}

	return nil, fmt.Errorf("failed to get a relevant response for %s: exceed max retries (%d)", command.String(), retryMaxAttempts)
}

func (l *xrealLight) GetSerial() (string, error) {
	response, err := l.executeAndWaitForResponse(newCommandPacket(&CMD_GET_SERIAL_NUMBER))
	if err != nil {
		return "", fmt.Errorf("failed to %s: %w", CMD_GET_SERIAL_NUMBER.String(), err)
	}
	return string(response), nil
}

func (l *xrealLight) GetFirmwareVersion() (string, error) {
	response, err := l.executeAndWaitForResponse(newCommandPacket(&CMD_GET_FIRMWARE_VERSION_0))
	if err != nil {
		return "", fmt.Errorf("failed to %s: %w", CMD_GET_FIRMWARE_VERSION_0.String(), err)
	}
	return string(response), nil
}

func (l *xrealLight) GetOptionsEnabled(options []string) []string {
	var results []string

	for _, option := range options {
		var packet *Packet

		switch option {
		case "ambientlight":
			packet = newCommandPacket(&CMD_GET_AMBIENT_LIGHT_ENABLED)
		case "vsync":
			packet = newCommandPacket(&CMD_GET_ENABLE_VSYNC_ENABLED)
		case "activated":
			packet = newCommandPacket(&CMD_GET_ACTIVATION)
		case "magnetometer":
			packet = newCommandPacket(&CMD_GET_MAGNETOMETER_ENABLED)
		default:
		}

		if packet == nil {
			results = append(results, "unknown option")
			continue
		}

		response, err := l.executeAndWaitForResponse(packet)
		if err != nil {
			results = append(results, fmt.Sprintf("failed to %s: %v", packet.Command.String(), err))

		} else {
			results = append(results, fmt.Sprintf("%s: %s", packet.Command.String(), string(response)))
		}
	}
	return results
}

func (l *xrealLight) GetDisplayMode() (DisplayMode, error) {
	response, err := l.executeAndWaitForResponse(newCommandPacket(&CMD_GET_DISPLAY_MODE))
	if err != nil {
		return DISPLAY_MODE_UNKNOWN, fmt.Errorf("failed to %s: %w", CMD_GET_DISPLAY_MODE.String(), err)
	}
	if response[0] == '1' {
		// "1&2D_1080"
		return DISPLAY_MODE_SAME_ON_BOTH, nil
	} else if response[0] == '2' {
		// "2&3D_540"
		return DISPLAY_MODE_HALF_SBS, nil
	} else if response[0] == '3' {
		// "3&3D_1080"
		return DISPLAY_MODE_STEREO, nil
	} else if response[0] == '4' {
		// "4&3D_1080#72"
		return DISPLAY_MODE_HIGH_REFRESH_RATE, nil
	}
	return DISPLAY_MODE_UNKNOWN, fmt.Errorf("unrecognized response: %s", response)
}

func (l *xrealLight) SetDisplayMode(mode DisplayMode) error {
	var displayMode uint8
	if mode == DISPLAY_MODE_SAME_ON_BOTH {
		displayMode = '1'
	} else if mode == DISPLAY_MODE_HALF_SBS {
		displayMode = '2'
	} else if mode == DISPLAY_MODE_STEREO {
		displayMode = '3'
	} else if mode == DISPLAY_MODE_HIGH_REFRESH_RATE {
		displayMode = '4'
	} else {
		return fmt.Errorf("unknown display mode: %v", mode)
	}

	packet := newCommandPacket(&CMD_SET_DISPLAY_MODE, []byte{displayMode})
	response, err := l.executeAndWaitForResponse(packet)
	if err != nil {
		return fmt.Errorf("failed to %s: %w", CMD_SET_DISPLAY_MODE.String(), err)
	}
	if response[0] != displayMode {
		return fmt.Errorf("failed to %s: want %d got %d", CMD_SET_DISPLAY_MODE.String(), displayMode, response[0])
	}
	return nil
}

func (l *xrealLight) GetBrightnessLevel() (string, error) {
	packet := newCommandPacket(&CMD_GET_BRIGHTNESS_LEVEL)
	if response, err := l.executeAndWaitForResponse(packet); err != nil {
		return "unknown", fmt.Errorf("failed to %s: %w", CMD_GET_BRIGHTNESS_LEVEL.String(), err)
	} else {
		return string(response), nil
	}
}

func (l *xrealLight) SetBrightnessLevel(level string) error {
	if (len(level) != 1) || (level[0] < '0') || (level[0] > '7') {
		return fmt.Errorf("invalid level %s, must be single digit 0-7", level)
	}

	packet := newCommandPacket(&CMD_SET_BRIGHTNESS_LEVEL_0, []byte(level))
	if response, err := l.executeAndWaitForResponse(packet); err != nil {
		return fmt.Errorf("failed to set brightness level: %w", err)
	} else if response[0] != level[0] {
		return fmt.Errorf("failed to set brightness mode: want %d got %s", level[0], string(response))
	}
	return nil
}

func (l *xrealLight) SetAmbientLightEventHandler(handler AmbientLightEventHandler) {
	l.deviceHandlers.AmbientLightEventHandler = handler
}

func (l *xrealLight) SetKeyEventHandler(handler KeyEventHandler) {
	l.deviceHandlers.KeyEventHandler = handler
}

func (l *xrealLight) SetMagnetometerEventHandler(handler MagnetometerEventHandler) {
	l.deviceHandlers.MagnetometerEventHandler = handler
}

func (l *xrealLight) SetProximityEventHandler(handler ProximityEventHandler) {
	l.deviceHandlers.ProximityEventHandler = handler
}

func (l *xrealLight) SetVSyncEventHandler(handler VSyncEventHandler) {
	l.deviceHandlers.VSyncEventHandler = handler
}

func (l *xrealLight) DevExecuteAndRead(input []string) {
	if len(input) != 3 {
		slog.Error(fmt.Sprintf("wrong input format: want [PacketType CommandID Payload] got %v", input))
		return
	}

	if len(input[1]) != 1 {
		slog.Error(fmt.Sprintf("wrong CommandID format: want ASCII char, got %s", input[1]))
		return
	}

	command := Command{Type: input[0][0], ID: input[1][0]}
	packet := newCommandPacket(&command, []byte(input[2]))
	response, err := l.executeAndWaitForResponse(packet)
	if err != nil {
		slog.Error(fmt.Sprintf("%v : '%s' failed: %v", command, string(response), err))
		return
	}
	slog.Info(fmt.Sprintf("%v : '%s'", command, string(response)))
}

func NewXREALLight(devicePath *string, serialNumber *string) Device {
	var l xrealLight
	if devicePath != nil {
		l.devicePath = devicePath
	}
	if serialNumber != nil {
		l.serialNumber = serialNumber
	}

	l.deviceHandlers = DeviceHandlers{
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
		VSyncEventHandler: func(vsync string) {
			slog.Info(fmt.Sprintf("VSync: %s", vsync))
		},
	}

	l.packetResponseChannel = make(chan *Packet)
	l.stopHeartBeatChannel = make(chan struct{})
	l.stopReadPacketsChannel = make(chan struct{})

	return &l
}
