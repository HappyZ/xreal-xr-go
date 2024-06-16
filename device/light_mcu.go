package device

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	hid "github.com/sstallion/go-hid"
)

const (
	// XREAL Light MCU
	XREAL_LIGHT_MCU_VID = uint16(0x0486)
	XREAL_LIGHT_MCU_PID = uint16(0x573c)
)

type xrealLightMCU struct {
	initialized bool

	device *hid.Device
	// serialNumber is optional and can be nil if not provided
	serialNumber *string
	// devicePath is optional and can be nil if not provided
	devicePath *string

	// deviceHandlers contains callback funcs for the events from the glass device
	deviceHandlers *DeviceHandlers

	// glassFirmware is obtained from mcuDevice and used to get the correct commands
	glassFirmware string

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

func (l *xrealLightMCU) connectAndInitialize() error {
	devices, err := EnumerateDevices(XREAL_LIGHT_MCU_VID, XREAL_LIGHT_MCU_PID)
	if err != nil {
		return fmt.Errorf("failed to enumerate MCU hid devices: %w", err)
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

	if l.devicePath != nil {
		if device, err := hid.OpenPath(*l.devicePath); err != nil {
			return fmt.Errorf("failed to open the device path %s: %w", *l.devicePath, err)
		} else {
			l.device = device
		}
	} else if l.serialNumber != nil {
		if device, err := hid.Open(XREAL_LIGHT_MCU_VID, XREAL_LIGHT_MCU_PID, *l.serialNumber); err != nil {
			return fmt.Errorf("failed to open the device with serial number %s: %w", *l.serialNumber, err)
		} else {
			l.device = device
		}
	} else {
		if device, err := hid.OpenFirst(XREAL_LIGHT_MCU_VID, XREAL_LIGHT_MCU_PID); err != nil {
			return fmt.Errorf("failed to open the first hid device for XREAL Light MCU: %w", err)
		} else {
			l.device = device
		}
	}

	// backfill missing data
	if info, err := l.device.GetDeviceInfo(); err == nil {
		l.devicePath = &info.Path
		l.serialNumber = &info.SerialNbr
	}

	return l.initialize()
}

func (l *xrealLightMCU) initialize() error {
	l.waitgroup.Add(1)
	go l.sendHeartBeatPeriodically()

	l.waitgroup.Add(1)
	go l.readPacketsPeriodically()

	// We must ensure we get the firmware version
	for {
		if firmwareVersion, err := getFirmwareVersion(l); err == nil {
			l.glassFirmware = firmwareVersion
			break
		}
	}

	// ensure glass is activated
	packet := l.buildCommandPacket(CMD_SET_GLASS_ACTIVATION, []byte("1"))
	for {
		if _, err := l.executeAndWaitForResponse(packet); err == nil {
			break
		}
	}

	// disable VSync event reporting by default with best effort
	l.enableEventReporting(CMD_ENABLE_VSYNC, "0")

	l.initialized = true

	return nil
}

func getFirmwareVersion(l *xrealLightMCU) (string, error) {
	packet := l.buildCommandPacket(CMD_GET_FIRMWARE_VERSION)
	response, err := l.executeAndWaitForResponse(packet)
	if err != nil {
		return "", fmt.Errorf("failed to %s: %w", packet.String(), err)
	}
	return string(response), nil
}

func (l *xrealLightMCU) sendHeartBeatPeriodically() {
	defer l.waitgroup.Done()

	ticker := time.NewTicker(heartBeatTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// don't do anything if not initialized
			if !l.initialized {
				continue
			}
			packet := l.buildCommandPacket(CMD_HEART_BEAT)
			if err := l.executeOnly(packet); err != nil {
				slog.Debug(fmt.Sprintf("failed to send a heartbeat: %v", err))
			}
		case <-l.stopHeartBeatChannel:
			return
		}
	}
}

// readPacketsPeriodically is a goroutine method to read info from XREAL Light MCU HID device
func (l *xrealLightMCU) readPacketsPeriodically() {
	defer l.waitgroup.Done()

	ticker := time.NewTicker(readPacketFrequency)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := l.readAndProcessPackets(); err != nil {
				if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "timed out") || strings.Contains(err.Error(), "system call") {
					continue
				}
				slog.Debug(fmt.Sprintf("readAndProcessPackets(): %v", err))
			}
		case <-l.stopReadPacketsChannel:
			return
		}
	}
}

func (l *xrealLightMCU) executeOnly(command *Packet) error {
	l.mutex.Lock()

	defer l.mutex.Unlock()

	if l.device == nil {
		return fmt.Errorf("not connected / initialized")
	}

	if serialized, err := command.Serialize(); err != nil {
		return fmt.Errorf("failed to serialize command %v: %w", command, err)
	} else {
		if _, err := l.device.Write(serialized[:]); err != nil {
			return fmt.Errorf("failed to execute on device %v: %w", l.device, err)
		}
	}
	return nil
}

// readAndProcessPackets sends a legit packet request to device and receives a set of packets to be processed.
// This method should be called as frequently as possible to track the time of the packets more accurately.
func (l *xrealLightMCU) readAndProcessPackets() error {
	packet := l.buildCommandPacket(CMD_GET_NREAL_FW_STRING)
	// we must send a packet to get all responses, which is a bit lame
	if err := l.executeOnly(packet); err != nil {
		return err
	}
	for i := 0; i < 32; i++ {
		var buffer [64]byte
		_, err := l.device.ReadWithTimeout(buffer[:], readDeviceTimeout)
		if err != nil {
			return fmt.Errorf("failed to read from device %v: %w", l.device, err)
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

		if (response.Command.Type == packet.Command.Type+1) && (response.Command.ID == packet.Command.ID) {
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
		if response.Type == PACKET_TYPE_MCU && l.initialized {
			if response.Command.EqualsInstruction(MCU_EVENT_KEY_PRESS) {
				switch string(response.Payload) {
				case "UP":
					l.deviceHandlers.KeyEventHandler(KEY_UP_PRESSED)
				case "DN":
					l.deviceHandlers.KeyEventHandler(KEY_DOWN_PRESSED)
				default:
					slog.Debug(fmt.Sprintf("Key pressed unrecognized: %s", string(response.Payload)))
					l.deviceHandlers.KeyEventHandler(KEY_UNKNOWN)
				}
			} else if response.Command.EqualsInstruction(MCU_EVENT_PROXIMITY) {
				switch string(response.Payload) {
				case "away":
					l.deviceHandlers.ProximityEventHandler(PROXIMITY_FAR)
				case "near":
					l.deviceHandlers.ProximityEventHandler(PROXIMITY_NEAR)
				default:
					slog.Info(fmt.Sprintf("Proximity unrecognized: %s", string(response.Payload)))
					l.deviceHandlers.ProximityEventHandler(PROXIMITY_UKNOWN)
				}
			} else if response.Command.EqualsInstruction(MCU_EVENT_AMBIENT_LIGHT) {
				if value, err := strconv.ParseUint(string(response.Payload), 10, 16); err != nil {
					slog.Debug(fmt.Sprintf("Ambient light failed to parse: %s", string(response.Payload)))
				} else {
					l.deviceHandlers.AmbientLightEventHandler(uint16(value))
				}
			} else if response.Command.EqualsInstruction(MCU_EVENT_VSYNC) {
				l.deviceHandlers.VSyncEventHandler(string(response.Payload))
			} else if response.Command.EqualsInstruction(MCU_EVENT_TEMPERATURE_A) || response.Command.EqualsInstruction(MCU_EVENT_TEMPERATURE_B) {
				l.deviceHandlers.TemperatureEventHandlder(string(response.Payload))
			} else if response.Command.EqualsInstruction(MCU_EVENT_MAGNETOMETER) {
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

func (l *xrealLightMCU) executeAndWaitForResponse(command *Packet) ([]byte, error) {
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
				continue
			}
			return nil, fmt.Errorf("failed to get response for %s: timed out", command.String())
		}
	}

	return nil, fmt.Errorf("failed to get a relevant response for %s: exceed max retries (%d)", command.String(), retryMaxAttempts)
}

func (l *xrealLightMCU) buildCommandPacket(instruction CommandInstruction, payload ...[]byte) *Packet {
	defaultPayload := []byte{' '}
	if len(payload) > 0 {
		defaultPayload = payload[0]
	}
	return &Packet{
		Type:      PACKET_TYPE_COMMAND,
		Command:   l.getCommand(instruction),
		Payload:   defaultPayload,
		Timestamp: getTimestampNow(),
	}
}

func (l *xrealLightMCU) getSerial() (string, error) {
	packet := l.buildCommandPacket(CMD_GET_SERIAL_NUMBER)
	response, err := l.executeAndWaitForResponse(packet)
	if err != nil {
		return "", fmt.Errorf("failed to %s: %w", packet.String(), err)
	}
	return string(response), nil
}

func (l *xrealLightMCU) getDisplayMode() (DisplayMode, error) {
	packet := l.buildCommandPacket(CMD_GET_DISPLAY_MODE)
	response, err := l.executeAndWaitForResponse(packet)
	if err != nil {
		return DISPLAY_MODE_UNKNOWN, fmt.Errorf("failed to %s: %w", packet.String(), err)
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

func (l *xrealLightMCU) setDisplayMode(mode DisplayMode) error {
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

	packet := l.buildCommandPacket(CMD_SET_DISPLAY_MODE, []byte{displayMode})
	response, err := l.executeAndWaitForResponse(packet)
	if err != nil {
		return fmt.Errorf("failed to %s: %w", packet.String(), err)
	}
	if response[0] != displayMode {
		return fmt.Errorf("failed to %s: want %d got %d", packet.String(), displayMode, response[0])
	}
	return nil
}

func (l *xrealLightMCU) getBrightnessLevel() (string, error) {
	packet := l.buildCommandPacket(CMD_GET_BRIGHTNESS_LEVEL)
	if response, err := l.executeAndWaitForResponse(packet); err != nil {
		return "unknown", fmt.Errorf("failed to %s: %w", packet.String(), err)
	} else {
		return string(response), nil
	}
}

func (l *xrealLightMCU) setBrightnessLevel(level string) error {
	if (len(level) != 1) || (level[0] < '0') || (level[0] > '7') {
		return fmt.Errorf("invalid level %s, must be single digit 0-7", level)
	}

	packet := l.buildCommandPacket(CMD_SET_BRIGHTNESS_LEVEL, []byte(level))
	if response, err := l.executeAndWaitForResponse(packet); err != nil {
		return fmt.Errorf("failed to set brightness level: %w", err)
	} else if response[0] != level[0] {
		return fmt.Errorf("failed to set brightness mode: want %s got %s", level, string(response))
	}
	return nil
}

func (l *xrealLightMCU) enableEventReporting(instruction CommandInstruction, enabled string) error {
	packet := l.buildCommandPacket(instruction, []byte(enabled))
	if response, err := l.executeAndWaitForResponse(packet); err != nil {
		return fmt.Errorf("failed to set event reporting: %w", err)
	} else if response[0] != enabled[0] {
		return fmt.Errorf("failed to set event reporting: want %s got %s", enabled, string(response))
	}
	return nil
}

func (l *xrealLightMCU) disconnect() error {
	l.initialized = false

	if l.device == nil {
		return nil
	}

	close(l.stopHeartBeatChannel)
	close(l.stopReadPacketsChannel)

	l.waitgroup.Wait()

	close(l.packetResponseChannel)

	err := l.device.Close()
	if err == nil {
		l.device = nil
	}

	// also cleans up whatever is initialized
	l.glassFirmware = ""

	return err
}

func (l *xrealLightMCU) devExecuteAndRead(input []string) {
	if len(input) != 3 {
		slog.Error(fmt.Sprintf("wrong input format: want [CommandType CommandID Payload] got %v", input))
		return
	}

	if len(input[1]) != 1 {
		slog.Error(fmt.Sprintf("wrong CommandID format: want ASCII char, got %s", input[1]))
		return
	}

	packet := &Packet{
		Type:      PACKET_TYPE_COMMAND,
		Command:   &Command{Type: input[0][0], ID: input[1][0]},
		Payload:   []byte(input[2]),
		Timestamp: getTimestampNow(),
	}
	response, err := l.executeAndWaitForResponse(packet)
	if err != nil {
		slog.Error(fmt.Sprintf("%v : '%s' failed: %v", packet.Command, string(response), err))
		return
	}
	slog.Info(fmt.Sprintf("%v : '%s'", packet.Command, string(response)))
}
