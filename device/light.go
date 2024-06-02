package device

import (
	"bytes"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"xreal-light-xr-go/constant"
	"xreal-light-xr-go/crc"

	hid "github.com/sstallion/go-hid"
)

const (
	readDeviceTimeout = 300 * time.Millisecond
	heartBeatTimeout  = 1 * time.Second
	retryMaxAttempts  = 5
)

// CommandID holds reverse engineered command ID info
type CommandID uint8

const (
	CMD_ID_BRIGHTNESS_LEVEL     CommandID = 0x31
	CMD_ID_DISPLAY_MODE         CommandID = 0x33
	CMD_ID_FW_VERSION           CommandID = 0x35
	CMD_ID_DEVICE_SERIAL_NUMBER CommandID = 0x43
	CMD_ID_AMBIENT_LIGHT_REPORT CommandID = 0x4c
	CMD_ID_V_SYNC_EVENT         CommandID = 0x4e
	CMD_ID_MAGNETOMETER_EVENT   CommandID = 0x55
	CMD_ID_GLASS_IS_ACTIVATED   CommandID = 0x65
	CMD_ID_ACTIVATION_TIME      CommandID = 0x66
	CMD_ID_CAM_RGB              CommandID = 0x68
	CMD_ID_CAM_STEREO           CommandID = 0x69
)

func (cmd CommandID) String() string {
	switch cmd {
	case CMD_ID_BRIGHTNESS_LEVEL:
		return "brightness level"
	case CMD_ID_DISPLAY_MODE:
		return "display mode"
	case CMD_ID_FW_VERSION:
		return "firmware version"
	case CMD_ID_DEVICE_SERIAL_NUMBER:
		return "glass serial number"
	case CMD_ID_AMBIENT_LIGHT_REPORT:
		return "ambient light reporting enabled"
	case CMD_ID_V_SYNC_EVENT:
		return "v-sync event enabled"
	case CMD_ID_MAGNETOMETER_EVENT:
		return "magnetometer event enabled"
	case CMD_ID_GLASS_IS_ACTIVATED:
		return "whether glass is activated"
	case CMD_ID_ACTIVATION_TIME:
		return "glass activation time"
	case CMD_ID_CAM_RGB:
		return "RGB camera enabled"
	case CMD_ID_CAM_STEREO:
		return "Stereo camera enabled"
	default:
		switch uint8(cmd) {
		case 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1c, 0x1d, 0x1e, 0x1f, 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f, 0x32, 0x38, 0x3b, 0x3c, 0x3d, 0x3e, 0x3f, 0x40, 0x4f, 0x54, 0x57, 0x5b, 0x5c, 0x5d, 0x5e, 0x5f, 0x63, 0x67, 0x6a, 0x6b, 0x77, 0x7b, 0x7c, 0x7d, 0x7e:
			return "no function"
		}
		return "unknown"
	}
}

// PacketType holds reverse engineered packet type info
type PacketType uint8

const (
	PKT_TYPE_GET PacketType = 0x33
	PKT_TYPE_SET PacketType = 0x31
)

func (pkttype PacketType) String() string {
	switch pkttype {
	case PKT_TYPE_GET:
		return "get"
	case PKT_TYPE_SET:
		return "set"
	default:
		return "unknown"
	}
}

type Packet struct {
	PacketType PacketType
	CommandID  CommandID
	Payload    []byte
	Timestamp  []byte
	// Only set if is CRC ERROR or unable to deserialize
	RawMessage string
}

func (pkt *Packet) DecodeTimestamp() time.Time {
	var t time.Time
	if (pkt.Timestamp == nil) || len(pkt.Timestamp) == 0 {
		return t
	}
	hexStr := string(pkt.Timestamp)
	milliseconds, err := strconv.ParseInt(hexStr, 16, 64)
	if err != nil {
		slog.Error(fmt.Sprintf("failed to parse %s (%v) to int64: %v", hexStr, pkt.Timestamp, err))
		return t
	}
	t = time.Unix(0, milliseconds*int64(time.Millisecond))
	return t
}

func (pkt *Packet) String() string {
	serialized, err := pkt.Serialize()
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s (at time %v)", string(serialized[:]), pkt.DecodeTimestamp())
}

func (pkt *Packet) Deserialize(data []byte) error {
	if data[0] == 'C' {
		// This is a CRC Error packet, e.g. "CAL CRC ERROR:20000614:200152e8"

		// endIdx := len(data) - 1
		// for i, b := range data {
		// 	if b == 0 {
		// 		break
		// 	}
		// 	endIdx = i
		// }

		// data = data[:endIdx-1]

		// parts := bytes.Split(data, []byte{':'})
		// if len(parts) < 3 {
		// 	return fmt.Errorf("input date carries with insufficient information")
		// }

		pkt.RawMessage = string(data)
		return nil
	}

	if data[0] != 0x02 {
		pkt.RawMessage = string(data)
		return fmt.Errorf("unrecognized data format")
	}

	endIdx := len(data) - 1
	for i, b := range data {
		if b == 3 {
			endIdx = i
		}
	}

	if data[endIdx] != 0x03 {
		return fmt.Errorf("invalid input data not ending with 0x03: %v", data)
	}

	// Removes start and end markers.
	data = data[2 : endIdx-1]

	parts := bytes.Split(data, []byte{':'})
	if len(parts) < 5 {
		return fmt.Errorf("input date carries with insufficient information")
	}

	pkt.PacketType = PacketType(parts[0][0])
	pkt.CommandID = CommandID(parts[1][0])
	pkt.Payload = parts[2]
	pkt.Timestamp = parts[len(parts)-2]

	return nil
}

// See https://voidcomputing.hu/blog/good-bad-ugly/#the-mcu-control-protocol.
func (pkt *Packet) Serialize() ([64]byte, error) {
	var result [64]byte

	var buf bytes.Buffer

	if pkt.RawMessage != "" {
		buf.Write([]byte(pkt.RawMessage))
	} else if (uint8(pkt.PacketType) == 0) || (uint8(pkt.CommandID) == 0) || (pkt.Payload == nil) || (pkt.Timestamp == nil) {
		return result, fmt.Errorf("this Packet is not initialized?")
	}

	buf.WriteByte(0x02)
	buf.WriteByte(':')
	buf.WriteByte(uint8(pkt.PacketType))
	buf.WriteByte(':')
	buf.WriteByte(uint8(pkt.CommandID))
	buf.WriteByte(':')
	buf.Write(pkt.Payload)
	buf.WriteByte(':')
	buf.Write(pkt.Timestamp)
	buf.WriteByte(':')
	crc := crc.CRC32(buf.Bytes())
	fmt.Fprintf(&buf, "%08x", crc)
	buf.WriteByte(':')
	buf.WriteByte(0x03)
	copy(result[:], buf.Bytes())

	return result, nil
}

const (
	// XREAL Light MCU
	XREAL_LIGHT_MCU_VID = uint16(0x0486)
	XREAL_LIGHT_MCU_PID = uint16(0x573c)
	// XREAL Light Camera and IMU
	XREAL_LIGHT_OV580_VID = uint16(0x05a9)
	XREAL_LIGHT_OV580_PID = uint16(0x0680)
)

type xrealLight struct {
	hidDevice *hid.Device
	// serialNumber is optional and can be nil if not provided
	serialNumber *string
	// devicePath is optional and can be nil if not provided
	devicePath *string

	// mutex for thread safety
	mutex sync.Mutex
	// waitgroup to wait for heart beat to stop sending
	waitgroup sync.WaitGroup
	// channel to signal heart beat to stop
	stopHeartBeatChannel chan struct{}
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
	if l.hidDevice == nil {
		return nil
	}

	// Sends signal to stop heart beat channel and wait for it
	close(l.stopHeartBeatChannel)
	l.waitgroup.Wait()

	// Properly closes the hid device opened
	err := l.hidDevice.Close()
	if err == nil {
		l.hidDevice = nil
	}
	return err
}

func (l *xrealLight) Connect() error {
	devices, err := enumerateDevices(XREAL_LIGHT_MCU_VID, XREAL_LIGHT_MCU_PID)
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

	var hidDevice *hid.Device

	if l.devicePath != nil {
		if device, err := hid.OpenPath(*l.devicePath); err != nil {
			return fmt.Errorf("failed to open the device path %s: %w", *l.devicePath, err)
		} else {
			hidDevice = device
		}
	} else if l.serialNumber != nil {
		if device, err := hid.Open(XREAL_LIGHT_MCU_VID, XREAL_LIGHT_MCU_PID, *l.serialNumber); err != nil {
			return fmt.Errorf("failed to open the device with serial number %s: %w", *l.serialNumber, err)
		} else {
			hidDevice = device
		}
	} else {
		if device, err := hid.OpenFirst(XREAL_LIGHT_MCU_VID, XREAL_LIGHT_MCU_PID); err != nil {
			return fmt.Errorf("failed to open the first hid device for XREAL Light MCU: %w", err)
		} else {
			hidDevice = device
		}
	}

	l.hidDevice = hidDevice

	return l.initialize()
}

func (l *xrealLight) initialize() error {
	// Initialize the stop channel
	l.stopHeartBeatChannel = make(chan struct{})
	l.waitgroup.Add(1)
	go l.sendPeriodicHeartBeat()

	// Disabled below because it is not necessary to send this if not in SBS mode
	// Sends an "SDK works" message
	// command := &Packet{PacketType: '@', CommandID: CMD_ID_DISPLAY_MODE, Payload: []byte{'1'}, Timestamp: getTimestampNow()}
	// err := l.executeOnly(command)
	// if err != nil {
	// 	return fmt.Errorf("failed to send SDK works message: %w", err)
	// }

	return nil
}

func (l *xrealLight) sendHeartBeat() error {
	command := &Packet{PacketType: '@', CommandID: 'K', Payload: []byte{'x'}, Timestamp: getTimestampNow()}
	err := l.executeOnly(command)
	if err != nil {
		return fmt.Errorf("failed to send a heart beat: %w", err)
	}
	return nil
}

func (l *xrealLight) sendPeriodicHeartBeat() {
	defer l.waitgroup.Done()

	ticker := time.NewTicker(heartBeatTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := l.sendHeartBeat()
			if err != nil {
				slog.Debug(fmt.Sprintf("failed to send a heartbeat: %v", err))
			}
		case <-l.stopHeartBeatChannel:
			return
		}
	}
}

func execute(device *hid.Device, serialized []byte) error {
	_, err := device.Write(serialized)
	if err != nil {
		return fmt.Errorf("failed to execute on device %v: %w", device, err)
	}
	return nil
}

func (l *xrealLight) executeOnly(command *Packet) error {
	l.mutex.Lock()

	defer l.mutex.Unlock()

	if serialized, err := command.Serialize(); err != nil {
		return fmt.Errorf("failed to serialize command %v: %w", command, err)
	} else {
		if err := execute(l.hidDevice, serialized[:]); err != nil {
			return fmt.Errorf("failed to send command %v: %w", command, err)
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

func (l *xrealLight) executeAndRead(command *Packet) ([]byte, error) {
	for retry := 0; retry < retryMaxAttempts; retry++ {
		if err := l.executeOnly(command); err != nil {
			return nil, err
		}

		for i := 0; i < 128; i++ {
			l.mutex.Lock()
			buffer, err := read(l.hidDevice, readDeviceTimeout)
			l.mutex.Unlock()
			if err != nil {
				slog.Debug(fmt.Sprintf("failed to read response: %v", err))
				break
			}

			response := &Packet{}

			if err := response.Deserialize(buffer[:]); err != nil {
				slog.Debug(fmt.Sprintf("failed to deserialize %v (%s): %v\n", buffer, string(buffer[:]), err))
				break
			}

			if (response.PacketType == command.PacketType+1) && (response.CommandID == command.CommandID) {
				return response.Payload, nil
			}
			// otherwise we received irrelevant data
			// TODO(happyz): Handles irrelevant data

			slog.Debug(fmt.Sprintf("got unhandled response %s", response.String()))
		}
	}
	return nil, fmt.Errorf("failed to get a relevant response")
}

func getTimestampNow() []byte {
	return []byte(fmt.Sprintf("%x", (time.Now().UnixMilli())))
}

func (l *xrealLight) GetSerial() (string, error) {
	command := &Packet{PacketType: PKT_TYPE_GET, CommandID: CMD_ID_DEVICE_SERIAL_NUMBER, Payload: []byte{'x'}, Timestamp: getTimestampNow()}
	response, err := l.executeAndRead(command)
	if err != nil {
		return "", fmt.Errorf("failed to get serial: %w", err)
	}
	return string(response), nil
}

func (l *xrealLight) GetFirmwareVersion() (string, error) {
	command := &Packet{PacketType: PKT_TYPE_GET, CommandID: CMD_ID_FW_VERSION, Payload: []byte{'x'}, Timestamp: getTimestampNow()}
	response, err := l.executeAndRead(command)
	if err != nil {
		return "", fmt.Errorf("failed to get firmware version: %w", err)
	}
	return string(response), nil
}

func (l *xrealLight) GetDisplayMode() (DisplayMode, error) {
	command := &Packet{PacketType: PKT_TYPE_GET, CommandID: CMD_ID_DISPLAY_MODE, Payload: []byte{'x'}, Timestamp: getTimestampNow()}
	response, err := l.executeAndRead(command)
	if err != nil {
		return DISPLAY_MODE_UNKNOWN, fmt.Errorf("failed to get display mode: %w", err)
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

	command := &Packet{PacketType: PKT_TYPE_SET, CommandID: CMD_ID_DISPLAY_MODE, Payload: []byte{displayMode}, Timestamp: getTimestampNow()}
	response, err := l.executeAndRead(command)
	if err != nil {
		return fmt.Errorf("failed to set display mode: %w", err)
	}
	if response[0] != displayMode {
		return fmt.Errorf("failed to set display mode: want %d got %d", displayMode, response[0])
	}
	return nil
}

func (l *xrealLight) PrintCommandIDTable() {
	slog.Info("=======================")
	slog.Info("PacketType : CommandId : Payload : Purpose : Output")
	slog.Info("=======================")
	// we loop through ASCII char that doesn't have special meanings
	for i := uint8(0x20); i < 0x7f; i++ {
		commandID := CommandID(i)
		// skip
		// if commandID.String() == "no function" {
		// 	continue
		// }
		command := &Packet{PacketType: PKT_TYPE_GET, CommandID: commandID, Payload: []byte{' '}, Timestamp: getTimestampNow()}
		response, err := l.executeAndRead(command)

		if err != nil {
			slog.Error(fmt.Sprintf("('%s' : '0x%x (%c)' : ' ' : %s : '%s') failed: %v", command.PacketType.String(), i, i, commandID.String(), string(response), err))
			continue
		}
		slog.Info(fmt.Sprintf("'%s' : '0x%x (%c)' : ' ' : %s : '%s'", command.PacketType.String(), i, i, commandID.String(), string(response)))
	}
	slog.Info("=======================")
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

	command := &Packet{CommandID: CommandID(input[1][0]), Payload: []byte(input[2]), Timestamp: getTimestampNow()}

	switch input[0] {
	case "get":
		command.PacketType = PKT_TYPE_GET
	case "set":
		command.PacketType = PKT_TYPE_SET
	default:
		if len(input[0]) == 1 {
			command.PacketType = PacketType(input[0][0])
		} else {
			slog.Error(fmt.Sprintf("unsupported PacketType %s", input[0]))
			return
		}
	}

	response, err := l.executeAndRead(command)

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
	return &l
}
