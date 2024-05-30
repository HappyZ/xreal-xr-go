package device

import (
	"bytes"
	"fmt"
	"time"

	"xreal-light-xr-go/crc"

	hid "github.com/sstallion/go-hid"
)

const (
	commandTimeout = 300 * time.Millisecond
)

type Command struct {
	CmdType   uint8
	CmdId     uint8
	Payload   []byte
	Timestamp uint8
}

func (c *Command) Deserialize(data []byte) error {
	if len(data) < 5 || data[0] != 2 || data[len(data)-1] != 3 {
		return fmt.Errorf("invalid input data")
	}

	// Removes start and end markers.
	data = data[2 : len(data)-2]

	parts := bytes.Split(data, []byte{':'})
	if len(parts) < 5 {
		return fmt.Errorf("input date carries with insufficient information")
	}

	c.CmdType = parts[0][0]
	c.CmdId = parts[1][0]
	c.Payload = parts[2]
	c.Timestamp = parts[len(parts)-2][0]
	return nil
}

func (c *Command) Serialize() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte(2)
	buf.WriteByte(':')
	buf.WriteByte(c.CmdType)
	buf.WriteByte(':')
	buf.WriteByte(c.CmdId)
	buf.WriteByte(':')
	buf.Write(c.Payload)
	buf.WriteByte(':')
	buf.WriteByte(c.Timestamp)
	buf.WriteByte(':')
	crc := crc.CRC32(buf.Bytes())
	fmt.Fprintf(&buf, "%08x", crc)
	buf.WriteByte(':')
	buf.WriteByte(3)
	return buf.Bytes(), nil
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

func (l *xrealLight) init() error {
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

func read(device *hid.Device, buffer [64]byte, timeout time.Duration) error {
	_, err := device.ReadWithTimeout(buffer[:], timeout)
	if err != nil {
		return fmt.Errorf("failed to read from device %v: %v", device, err)
	}
	return nil
}

func (l *xrealLight) executeAndRead(command *Command) ([]byte, error) {
	if serialized, err := command.Serialize(); err != nil {
		return nil, fmt.Errorf("failed to serialize command %v: %v", command, err)
	} else {
		if err := execute(l.hidDevice, serialized); err != nil {
			return nil, fmt.Errorf("failed to send command %v: %v", command, err)
		}
	}

	for i := 0; i < 128; i++ {
		var buffer [64]byte
		if err := read(l.hidDevice, buffer, commandTimeout); err != nil {
			return nil, fmt.Errorf("failed to read response: %v", err)
		}

		response := &Command{}
		if err := response.Deserialize(buffer[:]); err != nil {
			return nil, fmt.Errorf("failed to deserialize %v: %v", buffer, err)
		}
		if (response.CmdType == command.CmdType+1) && (response.CmdId == command.CmdId) {
			return response.Payload, nil
		}
		// otherwise we received irrelevant data
		// TODO(happyz): Handles irrelevant data
	}

	return nil, nil
}

func (l *xrealLight) GetDisplayMode() (DisplayMode, error) {
	command := &Command{CmdType: '3', CmdId: '3'}
	response, err := l.executeAndRead(command)
	if err != nil {
		return DISPLAY_MODE_UNKNOWN, fmt.Errorf("failed to get display mode: %v", err)
	}
	if len(response) != 1 {
		return DISPLAY_MODE_UNKNOWN, fmt.Errorf("invalid response on command %v: %v", command, response)
	}
	if response[0] == '1' {
		return DISPLAY_MODE_SAME_ON_BOTH, nil
	} else if response[0] == '2' {
		return DISPLAY_MODE_HALF_SBS, nil
	} else if response[0] == '3' {
		return DISPLAY_MODE_STEREO, nil
	} else if response[0] == '4' {
		return DISPLAY_MODE_HIGH_REFRESH_RATE, nil
	}
	return DISPLAY_MODE_UNKNOWN, fmt.Errorf("unrecognized response: %v", response)
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
		return fmt.Errorf("unsupported mode: %v", mode)
	}

	command := &Command{CmdType: '1', CmdId: '3', Payload: []byte{displayMode}}
	response, err := l.executeAndRead(command)
	if err != nil {
		return fmt.Errorf("failed to set display mode: %v", err)
	}
	if len(response) != 1 {
		return fmt.Errorf("invalid response on command %v: %v", command, response)
	}
	if response[0] != displayMode {
		return fmt.Errorf("failed to set display mode: want %d got %d", displayMode, response[0])
	}
	return nil
}

func NewXREALLight(devicePath *string) (Device, error) {
	var l xrealLight
	if devicePath != nil {
		l.devicePath = devicePath
	}
	return &l, l.init()
}
