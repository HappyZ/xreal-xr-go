package device

import (
	"bytes"
	"container/list"
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
	readDeviceTimeout   = 8 * time.Millisecond
	readPacketFrequency = 10 * time.Millisecond

	waitForPacketTimeout = 1 * time.Second
	retryMaxAttempts     = 3

	heartBeatTimeout = 500 * time.Millisecond
)

type Command struct {
	Type uint8
	ID   uint8
}

func (cmd Command) Equals(another *Command) bool {
	return (cmd.Type == another.Type) && (cmd.ID == another.ID)
}

func (cmd Command) String() string {
	switch cmd {
	case CMD_GET_STOCK_FIRMWARE_VERSION:
		return "get stock firmware version"
	case CMD_SET_BRIGHTNESS_LEVEL_0, CMD_SET_BRIGHTNESS_LEVEL_1:
		return "set brightness level"
	case CMD_GET_BRIGHTNESS_LEVEL:
		return "get brightness level"
	case CMD_SET_MAX_BRIGHTNESS_LEVEL:
		return "set max brightness level"
	case CMD_SET_DISPLAY_MODE:
		return "set display mode"
	case CMD_GET_DISPLAY_MODE:
		return "get display mode"
	case CMD_GET_DISPLAY_FIRMWARE:
		return "get display firmware version"
	case CMD_GET_FIRMWARE_VERSION_0, CMD_GET_FIRMWARE_VERSION_1:
		return "get firmware version"
	case CMD_GET_SERIAL_NUMBER:
		return "get glass serial number"
	case CMD_HEART_BEAT:
		return "send heart beat"
	case CMD_ENABLE_AMBIENT_LIGHT:
		return "enable ambient light reporting"
	case CMD_GET_AMBIENT_LIGHT_ENABLED:
		return "get if ambient light reporting enabled"
	case CMD_ENABLE_VSYNC:
		return "eanble v-sync reporting"
	case CMD_GET_ENABLE_VSYNC_ENABLED:
		return "get if v-sync reporting enabled"
	case CMD_ENABLE_MAGNETOMETER:
		return "enable geo magnetometer reporting"
	case CMD_GET_MAGNETOMETER_ENABLED:
		return "get if geo magnetometer reporting enabled"
	case CMD_UPDATE_DISPLAY_FW_UPDATE:
		return "update display to firmware update"
	case CMD_ENABLE_TEMPERATURE:
		return "enable temperature reporting"
	case CMD_GET_TEMPERATURE_ENABLED:
		return "get if temperature reporting enabled"
	case CMD_SET_OLED_BRIGHTNESS_LEVEL:
		return "set OLED brightness level" // not on light
	case CMD_GET_OLED_BRIGHTNESS_LEVEL:
		return "get OLED brightness level" // not on light
	case CMD_SET_ACTIVATION:
		return "set glass activation"
	case CMD_GET_ACTIVATION:
		return "get if glass activated"
	case CMD_SET_ACTIVATION_TIME:
		return "set glass activation time (epoch, sec)"
	case CMD_GET_ACTIVATION_TIME:
		return "get glass activation time (epoch, sec)"
	case CMD_ENABLE_RGB_CAMERA:
		return "enable RGB camera"
	case CMD_GET_RGB_CAMERA_ENABLED:
		return "get if RGB camera enabled"
	case CMD_ENABLE_STEREO_CAMERA:
		return "enable stereo camera"
	case CMD_GET_STEREO_CAMERA_ENABLED:
		return "get if stereo camera enabled"
	case CMD_GET_EEPROM_ADDR_VALUE:
		return "get EEPROM value at given address"
	case CMD_GET_NREAL_FW_STRING:
		return "always returns hardcoded string `NrealFW`"
	case CMD_GET_MCU_SERIES:
		return "always returns hardcoded string `STM32F413MGY6`"
	case CMD_GET_MCU_RAM_SIZE:
		return "always returns hardcoded string `RAM_320Kbytes`"
	case CMD_GET_MCU_ROM_SIZE:
		return "always returns hardcoded string `ROM_1.5Mbytes`"
	case CMD_SET_SDK_WORKS:
		return "set or unset SDK works"
	case CMD_GLASS_SLEEP:
		return "force glass to sleep (disconnect)"
	default:
		return "unknown / no function"
	}
}

type PacketType int

const (
	PACKET_TYPE_UNKNOWN PacketType = iota
	PACKET_TYPE_CRC_ERROR
	PACKET_TYPE_COMMAND
	PACKET_TYPE_RESPONSE
	PACKET_TYPE_MCU
	PACKET_TYPE_HEART_BEAT_RESPONSE
)

var (
	// FIRMWARE_05_1_08_021 only
	// CMD_SET_MAX_BRIGHTNESS_LEVEL     = Command{Type: 0x33, ID: 0x32} // shouldn't do anything, static, does not take any input
	// CMD_GET_DISPLAY_HDCP             = Command{Type: 0x33, ID: 0x34} // hardcoded "ELLA2_1224_HDCP"
	// FIRMWARE_05_1_08_021 and above
	CMD_GET_STOCK_FIRMWARE_VERSION   = Command{Type: 0x33, ID: 0x30}
	CMD_SET_BRIGHTNESS_LEVEL_0       = Command{Type: 0x31, ID: 0x31}
	CMD_GET_BRIGHTNESS_LEVEL         = Command{Type: 0x33, ID: 0x31}
	CMD_SET_DISPLAY_MODE             = Command{Type: 0x31, ID: 0x33}
	CMD_GET_DISPLAY_MODE             = Command{Type: 0x33, ID: 0x33}
	CMD_GET_DISPLAY_MODE_STRING      = Command{Type: 0x33, ID: 0x64} // not very useful given CMD_GET_DISPLAY_MODE
	CMD_GET_FIRMWARE_VERSION_0       = Command{Type: 0x33, ID: 0x35}
	CMD_GET_FIRMWARE_VERSION_1       = Command{Type: 0x33, ID: 0x61} // same as CMD_GET_FIRMWARE_VERSION_0
	CMD_SET_POWER                    = Command{Type: 0x31, ID: 0x39} // unknown purpose, input '0'/'1'
	CMD_GET_POWER                    = Command{Type: 0x33, ID: 0x39} // unknown purpose, default to '0'
	CMD_CLEAR_EEPROM_VALUE           = Command{Type: 0x31, ID: 0x41} // untested, input 4 byte eeprom address, set to 0xff
	CMD_GET_SERIAL_NUMBER            = Command{Type: 0x33, ID: 0x43}
	CMD_SET_APPROACH_PS_VALUE        = Command{Type: 0x31, ID: 0x44} // unknown purpose, input integer string
	CMD_GET_APPROACH_PS_VALUE        = Command{Type: 0x33, ID: 0x44} // unknown purpose, mine by default is 130
	CMD_SET_DISTANCE_PS_VALUE        = Command{Type: 0x31, ID: 0x45} // unknown purpose, input integer string
	CMD_GET_DISTANCE_PS_VALUE        = Command{Type: 0x33, ID: 0x45} // unknown purpose, mine by default is 110
	CMD_GET_DISPLAY_VERSION          = Command{Type: 0x33, ID: 0x46} // unknown purpose, mine by default is ELLA2_07.20
	CMD_GET_DISPLAY_DEBUG_DATA       = Command{Type: 0x33, ID: 0x6b} // unknown purpose
	CMD_SET_EEPROM_0X27_SOMETHING    = Command{Type: 0x31, ID: 0x47} // untested
	CMD_GET_EEPROM_0X27_SOMETHING    = Command{Type: 0x33, ID: 0x47} // untested
	CMD_GET_EEPROM_0X43_SOMETHING    = Command{Type: 0x33, ID: 0x48} // untested
	CMD_SET_EEPROM_0X43_SOMETHING    = Command{Type: 0x40, ID: 0x41} // untested
	CMD_SET_EEPROM_0X95_SOMETHING    = Command{Type: 0x31, ID: 0x50} // untested
	CMD_REBOOT_GLASS                 = Command{Type: 0x31, ID: 0x52}
	CMD_SET_EEPROM_0X110_SOMETHING   = Command{Type: 0x40, ID: 0x53} // untested
	CMD_GET_EEPROM_ADDR_VALUE        = Command{Type: 0x33, ID: 0x4b}
	CMD_GET_ORBIT_FUNC               = Command{Type: 0x33, ID: 0x37} // unknown purpose
	CMD_SET_ORBIT_FUNC               = Command{Type: 0x40, ID: 0x34} // input 0x0b (open) or others (close)
	CMD_SET_OLED_LEFT_HORIZONTAL     = Command{Type: 0x31, ID: 0x48} // unknown purpose, input is integer 0-255
	CMD_SET_OLED_LEFT_VERTICAL       = Command{Type: 0x31, ID: 0x49} // unknown purpose, input is integer 0-255
	CMD_SET_OLED_RIGHT_HORIZONTAL    = Command{Type: 0x31, ID: 0x4a} // unknown purpose, input is integer 0-255
	CMD_SET_OLED_RIGHT_VERTICAL      = Command{Type: 0x31, ID: 0x4b} // unknown purpose, input is integer 0-255
	CMD_GET_OLED_LRHV_VALUE          = Command{Type: 0x33, ID: 0x4a} // unknown purpose, LH-LV-RH-RV values set above, mine default with 'L05L06R27R26'
	MCU_KEY_PRESS                    = Command{Type: 0x35, ID: 0x4b}
	CMD_ENABLE_AMBIENT_LIGHT         = Command{Type: 0x31, ID: 0x4c}
	CMD_GET_AMBIENT_LIGHT_ENABLED    = Command{Type: 0x33, ID: 0x4c}
	MCU_AMBIENT_LIGHT                = Command{Type: 0x35, ID: 0x4c}
	CMD_SET_DUTY                     = Command{Type: 0x31, ID: 0x4d} // affect display brightness, input is integer 0-100
	CMD_GET_DUTY                     = Command{Type: 0x33, ID: 0x4d}
	CMD_ENABLE_VSYNC                 = Command{Type: 0x31, ID: 0x4e} // input '0'/'1'
	CMD_GET_ENABLE_VSYNC_ENABLED     = Command{Type: 0x33, ID: 0x4e} // mine default with 1
	MCU_VSYNC                        = Command{Type: 0x35, ID: 0x53}
	MCU_PROXIMITY                    = Command{Type: 0x35, ID: 0x50}
	CMD_SET_SLEEP_TIME               = Command{Type: 0x31, ID: 0x51} // input is integer that's larger than 20
	CMD_GET_SLEEP_TIME               = Command{Type: 0x33, ID: 0x51} // mine by default is 60
	CMD_GET_GLASS_START_UP_NUM       = Command{Type: 0x33, ID: 0x52} // unknown purpose
	CMD_GET_GLASS_ERROR_NUM          = Command{Type: 0x54, ID: 0x46} // unknown purpose
	CMD_GLASS_SLEEP                  = Command{Type: 0x54, ID: 0x47}
	CMD_GET_SOME_VALUE               = Command{Type: 0x33, ID: 0x53} // unknown purpose, output a digit
	CMD_RESET_OV580                  = Command{Type: 0x31, ID: 0x54} // untested
	CMD_ENABLE_MAGNETOMETER          = Command{Type: 0x31, ID: 0x55} // input '0'/'1'
	CMD_GET_MAGNETOMETER_ENABLED     = Command{Type: 0x33, ID: 0x55}
	CMD_READ_MAGNETOMETER            = Command{Type: 0x54, ID: 0x45} // untested
	MCU_MAGNETOMETER                 = Command{Type: 0x35, ID: 0x4d}
	CMD_GET_NREAL_FW_STRING          = Command{Type: 0x33, ID: 0x56} // hardcoded string `NrealFW`
	CMD_GET_MCU_SERIES               = Command{Type: 0x33, ID: 0x58} // hardcoded string `STM32F413MGY6`
	CMD_GET_MCU_ROM_SIZE             = Command{Type: 0x33, ID: 0x59} // hardcoded string `ROM_1.5Mbytes`
	CMD_GET_MCU_RAM_SIZE             = Command{Type: 0x33, ID: 0x5a} // hardcoded string `RAM_320Kbytes`
	CMD_UPDATE_DISPLAY_FW_UPDATE     = Command{Type: 0x31, ID: 0x58} // dont do this to light, it bricks my dev glasses
	CMD_SET_BRIGHTNESS_LEVEL_1       = Command{Type: 0x31, ID: 0x59} // caution: upon testing it doesn't do what's expected in newer firmware, see https://github.com/badicsalex/ar-drivers-rs/issues/14#issuecomment-2148616976
	CMD_ENABLE_TEMPERATURE           = Command{Type: 0x31, ID: 0x60} // untested, input '0'/'1'
	CMD_GET_TEMPERATURE_ENABLED      = Command{Type: 0x33, ID: 0x60} // untested, guessed
	CMD_SET_OLED_BRIGHTNESS_LEVEL    = Command{Type: 0x31, ID: 0x62} // untested, input '0'/'1'
	CMD_GET_OLED_BRIGHTNESS_LEVEL    = Command{Type: 0x33, ID: 0x62} // untested
	CMD_SET_ACTIVATION               = Command{Type: 0x31, ID: 0x65} // untested, input '0'/'1'
	CMD_GET_ACTIVATION               = Command{Type: 0x33, ID: 0x65}
	CMD_SET_ACTIVATION_TIME          = Command{Type: 0x31, ID: 0x66} // untested, input 8 bytes (up to epoch seconds)
	CMD_GET_ACTIVATION_TIME          = Command{Type: 0x33, ID: 0x66}
	CMD_SET_SUPER_ACTIVE             = Command{Type: 0x31, ID: 0x67} // unknown, input '0'/'1'
	CMD_ENABLE_RGB_CAMERA            = Command{Type: 0x31, ID: 0x68} // untested, input '0'/'1'
	CMD_POWER_OFF_RGB_CAMERA         = Command{Type: 0x54, ID: 0x56} // untested
	CMD_POWER_ON_RGB_CAMERA          = Command{Type: 0x54, ID: 0x57} // untested
	CMD_GET_RGB_CAMERA_ENABLED       = Command{Type: 0x33, ID: 0x68}
	CMD_ENABLE_STEREO_CAMERA         = Command{Type: 0x31, ID: 0x69} // untested, input '0'/'1', OV580
	CMD_GET_STEREO_CAMERA_ENABLED    = Command{Type: 0x33, ID: 0x69}
	CMD_SET_DEBUG_LOG                = Command{Type: 0x40, ID: 0x31} // untested, input 0x08 (Usart) / 0x07 (CRC) / 0 disable both
	CMD_CHECK_SONY_OTP_STUFF         = Command{Type: 0x40, ID: 0x32} // untested
	CMD_SET_SDK_WORKS                = Command{Type: 0x40, ID: 0x33} // input '0'/'1'
	CMD_MCU_B_JUMP_TO_A              = Command{Type: 0x40, ID: 0x38} // untested, for firmware update
	CMD_MCU_UPDATE_FW_ON_A_START     = Command{Type: 0x40, ID: 0x39} // untested, for firmware update
	CMD_DEFAULT_2D_FUNC_ENABLE       = Command{Type: 0x40, ID: 0x46} // unknown
	CMD_KEYSWITCH_ENABLE             = Command{Type: 0x40, ID: 0x48} // unknown
	CMD_HEART_BEAT                   = Command{Type: 0x40, ID: 0x4b}
	CMD_UPDATE_DISPLAY_SUCCESS       = Command{Type: 0x40, ID: 0x4d} // this doesn't do much
	CMD_MCU_A_JUMP_TO_B              = Command{Type: 0x40, ID: 0x52} // untested, for firmware update
	CMD_DATA_KEY_SOMETHING           = Command{Type: 0x40, ID: 0x52} // unknown, input '1'-'6' does different things
	CMD_SET_LIGHT_COMPENSATION       = Command{Type: 0x46, ID: 0x47} // untested
	CMD_CALIBRATE_LIGHT_COMPENSATION = Command{Type: 0x54, ID: 0x51} // untested
	CMD_RETRY_GET_OTP                = Command{Type: 0x54, ID: 0x52} // untested
	CMD_GET_OLED_BRIGHTNESS_BRIT     = Command{Type: 0x54, ID: 0x55} // untested
	// FIRMWARE_05_5_08_059 only
	CMD_SET_MAX_BRIGHTNESS_LEVEL = Command{Type: 0x31, ID: 0x32} // shouldn't do anything, static, does not take any input
	CMD_GET_DISPLAY_FIRMWARE     = Command{Type: 0x33, ID: 0x34} // "ELLA2_0518_V017"
	CMD_GET_DISPLAY_HDCP         = Command{Type: 0x33, ID: 0x48} // "ELLA2_1224_HDCP"
)

type Packet struct {
	Type      PacketType
	Command   *Command
	Payload   []byte
	Timestamp []byte
	Message   string
}

var DUMMY_PAYLOAD = []byte{' '}

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
		pkt.Type = PACKET_TYPE_CRC_ERROR
		pkt.Message = string(data)
		return nil
	}

	if data[0] != 0x02 {
		pkt.Message = string(data)
		pkt.Type = PACKET_TYPE_UNKNOWN
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

	pkt.Command = &Command{Type: parts[0][0], ID: parts[1][0]}
	pkt.Payload = parts[2]

	if pkt.Command.Type == 0x32 || pkt.Command.Type == 0x34 || pkt.Command.Type == 0x41 || pkt.Command.Type == 0x55 {
		if pkt.Command.Type == 0x41 && pkt.Command.ID == 0x4b {
			pkt.Type = PACKET_TYPE_HEART_BEAT_RESPONSE
		} else {
			pkt.Type = PACKET_TYPE_RESPONSE
		}
		pkt.Timestamp = parts[len(parts)-2]
	} else if pkt.Command.Type == 0x31 || pkt.Command.Type == 0x33 || pkt.Command.Type == 0x40 || pkt.Command.Type == 0x54 {
		pkt.Type = PACKET_TYPE_COMMAND
		pkt.Timestamp = parts[len(parts)-2]
	} else if pkt.Command.Type == 0x35 {
		if pkt.Command.ID == 0x4b || pkt.Command.ID == 0x4c || pkt.Command.ID == 0x4d || pkt.Command.ID == 0x50 {
			pkt.Type = PACKET_TYPE_MCU
		} else {
			pkt.Type = PACKET_TYPE_UNKNOWN
		}
		pkt.Message = string(data)
		pkt.Timestamp = getTimestampNow()
	} else {
		pkt.Type = PACKET_TYPE_UNKNOWN
		pkt.Message = string(data)
		pkt.Timestamp = getTimestampNow()
	}

	return nil
}

// See https://voidcomputing.hu/blog/good-bad-ugly/#the-mcu-control-protocol.
func (pkt *Packet) Serialize() ([64]byte, error) {
	var result [64]byte

	var buf bytes.Buffer

	if pkt.Type == PACKET_TYPE_CRC_ERROR || pkt.Type == PACKET_TYPE_UNKNOWN || pkt.Type == PACKET_TYPE_MCU {
		if pkt.Message != "" {
			buf.Write([]byte(pkt.Message))
			return result, nil
		}
		return result, fmt.Errorf("this Packet does not contain Message")
	}

	// All other types should have the following four fields
	if (uint8(pkt.Command.Type) == 0) || (uint8(pkt.Command.ID) == 0) || (pkt.Payload == nil) || (pkt.Timestamp == nil) {
		return result, fmt.Errorf("this Packet is not initialized?")
	}

	buf.WriteByte(0x02)
	buf.WriteByte(':')
	buf.WriteByte(uint8(pkt.Command.Type))
	buf.WriteByte(':')
	buf.WriteByte(uint8(pkt.Command.ID))
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

	// queue for Packet processing
	queue *list.List
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
	if l.hidDevice == nil {
		return nil
	}

	close(l.stopHeartBeatChannel)
	close(l.stopReadPacketsChannel)

	l.waitgroup.Wait()

	close(l.packetResponseChannel)

	err := l.hidDevice.Close()
	if err == nil {
		l.hidDevice = nil
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
	l.packetResponseChannel = make(chan *Packet)

	l.stopHeartBeatChannel = make(chan struct{})
	l.waitgroup.Add(1)
	go l.sendPeriodicHeartBeat()

	l.stopReadPacketsChannel = make(chan struct{})
	l.waitgroup.Add(1)
	go l.readPacketsPeriodically()

	return nil
}

func (l *xrealLight) sendHeartBeat() error {
	command := &Packet{Type: PACKET_TYPE_COMMAND, Command: &CMD_HEART_BEAT, Payload: DUMMY_PAYLOAD, Timestamp: getTimestampNow()}
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
			return err
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
	packet := &Packet{Type: PACKET_TYPE_COMMAND, Command: &CMD_GET_NREAL_FW_STRING, Payload: DUMMY_PAYLOAD, Timestamp: getTimestampNow()}
	// we must send a packet to get all responses, which is a bit lame
	if err := l.executeOnly(packet); err != nil {
		return err
	}
	for i := 0; i < 32; i++ {
		buffer, err := read(l.hidDevice, readDeviceTimeout)
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
		if response.Type == PACKET_TYPE_MCU {
			if response.Command.Equals(&MCU_KEY_PRESS) {
				slog.Info(fmt.Sprintf("Key pressed: %s", string(response.Payload)))
			} else if response.Command.Equals(&MCU_PROXIMITY) {
				slog.Info(fmt.Sprintf("Proximity: %s", string(response.Payload)))
			} else if response.Command.Equals(&MCU_AMBIENT_LIGHT) {
				slog.Info(fmt.Sprintf("Ambient Light: %s", string(response.Payload)))
			}
			// VSync has too many messages, cannot print
			continue
		}

		slog.Debug(fmt.Sprintf("got unhandled packet: %s", response.String()))
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

func getTimestampNow() []byte {
	return []byte(fmt.Sprintf("%x", (time.Now().UnixMilli())))
}

func (l *xrealLight) GetSerial() (string, error) {
	packet := &Packet{Type: PACKET_TYPE_COMMAND, Command: &CMD_GET_SERIAL_NUMBER, Payload: DUMMY_PAYLOAD, Timestamp: getTimestampNow()}
	response, err := l.executeAndWaitForResponse(packet)
	if err != nil {
		return "", fmt.Errorf("failed to %s: %w", CMD_GET_SERIAL_NUMBER.String(), err)
	}
	return string(response), nil
}

func (l *xrealLight) GetFirmwareVersion() (string, error) {
	packet := &Packet{Type: PACKET_TYPE_COMMAND, Command: &CMD_GET_FIRMWARE_VERSION_0, Payload: DUMMY_PAYLOAD, Timestamp: getTimestampNow()}
	response, err := l.executeAndWaitForResponse(packet)
	if err != nil {
		return "", fmt.Errorf("failed to %s: %w", CMD_GET_FIRMWARE_VERSION_0.String(), err)
	}
	return string(response), nil
}

func (l *xrealLight) GetDisplayMode() (DisplayMode, error) {
	packet := &Packet{Type: PACKET_TYPE_COMMAND, Command: &CMD_GET_DISPLAY_MODE, Payload: DUMMY_PAYLOAD, Timestamp: getTimestampNow()}
	response, err := l.executeAndWaitForResponse(packet)
	if err != nil {
		return DISPLAY_MODE_UNKNOWN, fmt.Errorf("failed to %s: %w", CMD_GET_DISPLAY_MODE.String(), err)
	}
	slog.Debug(fmt.Sprintf("%v responds %s", packet, string(response)))
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

	packet := &Packet{Type: PACKET_TYPE_COMMAND, Command: &CMD_SET_DISPLAY_MODE, Payload: []byte{displayMode}, Timestamp: getTimestampNow()}
	response, err := l.executeAndWaitForResponse(packet)
	if err != nil {
		return fmt.Errorf("failed to %s: %w", CMD_SET_DISPLAY_MODE.String(), err)
	}
	slog.Debug(fmt.Sprintf("%v responds %s", packet, string(response)))
	if response[0] != displayMode {
		return fmt.Errorf("failed to %s: want %d got %d", CMD_SET_DISPLAY_MODE.String(), displayMode, response[0])
	}
	return nil
}

func (l *xrealLight) GetBrightnessLevel() (string, error) {
	packet := &Packet{Type: PACKET_TYPE_COMMAND, Command: &CMD_GET_BRIGHTNESS_LEVEL, Payload: DUMMY_PAYLOAD, Timestamp: getTimestampNow()}
	if response, err := l.executeAndWaitForResponse(packet); err != nil {
		return "unknown", fmt.Errorf("failed to %s: %w", CMD_GET_BRIGHTNESS_LEVEL.String(), err)
	} else if response[0] == '0' {
		return "dimmest", nil
	} else if response[0] == '7' { // CMD_TOTAL_BRIGHTNESS_LEVELS tells there are 8
		return "brightest", nil
	} else {
		return string(response), nil
	}
}

func (l *xrealLight) SetBrightnessLevel(level string) error {
	if (len(level) != 1) || (level[0] < '0') || (level[0] > '7') {
		return fmt.Errorf("invalid level %s, must be 0-7", level)
	}

	command := &Packet{Type: PACKET_TYPE_COMMAND, Command: &CMD_SET_BRIGHTNESS_LEVEL_0, Payload: []byte(level), Timestamp: getTimestampNow()}
	response, err := l.executeAndWaitForResponse(command)
	if err != nil {
		return fmt.Errorf("failed to set brightness level: %w", err)
	}
	slog.Debug(fmt.Sprintf("%v responds %s", command, string(response)))
	if response[0] != level[0] {
		return fmt.Errorf("failed to set brightness mode: want %d got %d", level[0], response[0])
	}
	return nil
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
	packet := &Packet{Type: PACKET_TYPE_COMMAND, Command: &command, Payload: []byte(input[2]), Timestamp: getTimestampNow()}

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
	return &l
}
