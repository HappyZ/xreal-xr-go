package device

import (
	"bytes"
	"fmt"
	"strconv"
	"time"

	"xreal-light-xr-go/crc"

	hid "github.com/sstallion/go-hid"
)

const (
	commandTimeout = 1 * time.Second
)

type Packet struct {
	PacketType uint8
	CmdId      uint8
	Payload    []byte
	Timestamp  []byte
}

func (pkt *Packet) Deserialize(data []byte) error {
	if len(data) < 5 || data[0] != 0x02 {
		return fmt.Errorf("invalid input data not starting with 0x02: %v", data)
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

	pkt.PacketType = parts[0][0]
	pkt.CmdId = parts[1][0]
	pkt.Payload = parts[2]
	pkt.Timestamp = parts[len(parts)-2]

	return nil
}

// See https://voidcomputing.hu/blog/good-bad-ugly/#the-mcu-control-protocol.
func (pkt *Packet) Serialize() ([64]byte, error) {
	var result [64]byte

	var buf bytes.Buffer

	buf.WriteByte(0x02)
	buf.WriteByte(':')
	buf.WriteByte(pkt.PacketType)
	buf.WriteByte(':')
	buf.WriteByte(pkt.CmdId)
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
}

func (l *xrealLight) Name() string {
	return "XREAL LIGHT"
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
	err := l.hidDevice.Close()
	if err == nil {
		l.hidDevice = nil
	}
	return err
}

func (l *xrealLight) Connect() error {
	devices, err := enumerateDevices(XREAL_LIGHT_MCU_VID, XREAL_LIGHT_MCU_PID)
	if err != nil {
		fmt.Printf("failed to enumerate hid devices: %v\n", err)
		return err
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
			return fmt.Errorf("failed to open the device path %s: %v", *l.devicePath, err)
		} else {
			hidDevice = device
		}
	} else if l.serialNumber != nil {
		if device, err := hid.Open(XREAL_LIGHT_MCU_VID, XREAL_LIGHT_MCU_PID, *l.serialNumber); err != nil {
			return fmt.Errorf("failed to open the device with serial number %s: %v", *l.serialNumber, err)
		} else {
			hidDevice = device
		}
	} else {
		if device, err := hid.OpenFirst(XREAL_LIGHT_MCU_VID, XREAL_LIGHT_MCU_PID); err != nil {
			return fmt.Errorf("failed to open the first hid device for XREAL Light MCU: %v", err)
		} else {
			hidDevice = device
		}
	}

	l.hidDevice = hidDevice

	return nil
}

func execute(device *hid.Device, serialized []byte) error {
	_, err := device.Write(serialized)
	if err != nil {
		return fmt.Errorf("failed to execute on device %v: %v", device, err)
	}
	return nil
}

func (l *xrealLight) executeOnly(command *Packet) error {
	if serialized, err := command.Serialize(); err != nil {
		return fmt.Errorf("failed to serialize command %v: %v", command, err)
	} else {
		if err := execute(l.hidDevice, serialized[:]); err != nil {
			return fmt.Errorf("failed to send command %v: %v", command, err)
		}
	}
	return nil
}

func read(device *hid.Device, timeout time.Duration) ([64]byte, error) {
	var buffer [64]byte

	_, err := device.ReadWithTimeout(buffer[:], timeout)
	if err != nil {
		return buffer, fmt.Errorf("failed to read from device %v: %v", device, err)
	}

	return buffer, nil
}

func (l *xrealLight) executeAndRead(command *Packet) ([]byte, error) {
	if err := l.executeOnly(command); err != nil {
		return nil, err
	}

	for i := 0; i < 128; i++ {
		buffer, err := read(l.hidDevice, commandTimeout)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %v", err)
		}

		response := &Packet{}
		if err := response.Deserialize(buffer[:]); err != nil {
			fmt.Printf("failed to deserialize %v (%s): %v\n", buffer, string(buffer[:]), err)
			continue
		}
		if (response.PacketType == command.PacketType+1) && (response.CmdId == command.CmdId) {
			return response.Payload, nil
		}
		// otherwise we received irrelevant data
		// TODO(happyz): Handles irrelevant data
	}

	return nil, nil
}

func getTimestampNow() []byte {
	return []byte(strconv.FormatInt(time.Now().Unix(), 10))
}

func (l *xrealLight) GetSerial() (string, error) {
	command := &Packet{PacketType: '3', CmdId: 'C', Payload: []byte{'x'}, Timestamp: getTimestampNow()}
	response, err := l.executeAndRead(command)
	if err != nil {
		return "", fmt.Errorf("failed to get serial: %v", err)
	}
	return string(response), nil
}

func (l *xrealLight) GetFirmwareVersion() (string, error) {
	command := &Packet{PacketType: '3', CmdId: '5', Payload: []byte{'x'}, Timestamp: getTimestampNow()}
	response, err := l.executeAndRead(command)
	if err != nil {
		return "", fmt.Errorf("failed to get firmware version: %v", err)
	}
	return string(response), nil
}

func (l *xrealLight) GetDisplayMode() (DisplayMode, error) {
	command := &Packet{PacketType: '3', CmdId: '3', Payload: []byte{'x'}, Timestamp: getTimestampNow()}
	response, err := l.executeAndRead(command)
	if err != nil {
		return DISPLAY_MODE_UNKNOWN, fmt.Errorf("failed to get display mode: %v", err)
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

	command := &Packet{PacketType: '1', CmdId: '3', Payload: []byte{displayMode}, Timestamp: getTimestampNow()}
	response, err := l.executeAndRead(command)
	if err != nil {
		return fmt.Errorf("failed to set display mode: %v", err)
	}
	if response[0] != displayMode {
		return fmt.Errorf("failed to set display mode: want %d got %d", displayMode, response[0])
	}
	return nil
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
