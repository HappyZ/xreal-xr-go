package device

import (
	"bytes"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"xreal-light-xr-go/crc"

	hid "github.com/sstallion/go-hid"
)

const (
	readDeviceTimeout      = 300 * time.Millisecond
	writeReadDelay         = 100 * time.Millisecond
	heartBeatTimeout       = 1 * time.Second
	retryMaxAttempts       = 5
	retryBackoffIncrements = 100 * time.Millisecond
)

type Packet struct {
	PacketType uint8
	CmdId      uint8
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

	if pkt.RawMessage != "" {
		buf.Write([]byte(pkt.RawMessage))
	} else if (pkt.PacketType == 0) || (pkt.CmdId == 0) || (pkt.Payload == nil) || (pkt.Timestamp == nil) {
		return result, fmt.Errorf("this Packet is not initialized?")
	}

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

	// mutex for thread safety
	mutex sync.Mutex
	// waitgroup to wait for heart beat to stop sending
	waitgroup sync.WaitGroup
	// channel to signal heart beat to stop
	stopHeartBeatChannel chan struct{}
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

	// Sends an "SDK works" message
	command := &Packet{PacketType: '@', CmdId: '3', Payload: []byte{'1'}, Timestamp: getTimestampNow()}
	err := l.executeOnly(command)
	if err != nil {
		return fmt.Errorf("failed to send SDK works message: %w", err)
	}

	return nil
}

func (l *xrealLight) sendHeartBeat() error {
	command := &Packet{PacketType: '@', CmdId: 'K', Payload: []byte{'x'}, Timestamp: getTimestampNow()}
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
				slog.Warn(fmt.Sprintf("failed to send a heartbeat: %v", err))
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
	backoffTime := writeReadDelay

	for retry := 0; retry < retryMaxAttempts; retry++ {
		if err := l.executeOnly(command); err != nil {
			return nil, err
		}

		fmt.Println("sleep backoffTime", backoffTime)
		time.Sleep(backoffTime)

		for i := 0; i < 128; i++ {
			buffer, err := read(l.hidDevice, readDeviceTimeout)
			if err != nil {
				slog.Debug(fmt.Sprintf("failed to read response: %v", err))
				backoffTime += retryBackoffIncrements
				break
			}

			response := &Packet{}

			if err := response.Deserialize(buffer[:]); err != nil {
				slog.Debug(fmt.Sprintf("failed to deserialize %v (%s): %v\n", buffer, string(buffer[:]), err))
				backoffTime += retryBackoffIncrements
				break
			}

			if (response.PacketType == command.PacketType+1) && (response.CmdId == command.CmdId) {
				return response.Payload, nil
			}
			// otherwise we received irrelevant data
			// TODO(happyz): Handles irrelevant data

			fmt.Println("response", response.String())
			fmt.Println("sleep retryBackoffIncrements", retryBackoffIncrements)
			// time.Sleep(retryBackoffIncrements)
		}
	}
	return nil, nil
}

func getTimestampNow() []byte {
	return []byte(fmt.Sprintf("%x", (time.Now().UnixMilli())))
}

func (l *xrealLight) GetSerial() (string, error) {
	command := &Packet{PacketType: '3', CmdId: 'C', Payload: []byte{'x'}, Timestamp: getTimestampNow()}
	response, err := l.executeAndRead(command)
	if err != nil {
		return "", fmt.Errorf("failed to get serial: %w", err)
	}
	return string(response), nil
}

func (l *xrealLight) GetFirmwareVersion() (string, error) {
	command := &Packet{PacketType: '3', CmdId: '5', Payload: []byte{'x'}, Timestamp: getTimestampNow()}
	response, err := l.executeAndRead(command)
	if err != nil {
		return "", fmt.Errorf("failed to get firmware version: %w", err)
	}
	return string(response), nil
}

func (l *xrealLight) GetDisplayMode() (DisplayMode, error) {
	command := &Packet{PacketType: '3', CmdId: '3', Payload: []byte{'x'}, Timestamp: getTimestampNow()}
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

	command := &Packet{PacketType: '1', CmdId: '3', Payload: []byte{displayMode}, Timestamp: getTimestampNow()}
	response, err := l.executeAndRead(command)
	if err != nil {
		return fmt.Errorf("failed to set display mode: %w", err)
	}
	if response[0] != displayMode {
		return fmt.Errorf("failed to set display mode: want %d got %d", displayMode, response[0])
	}
	return nil
}

func (l *xrealLight) PrintExhaustiveCommandTable() error {
	slog.Debug("")
	slog.Debug("PacketType : CommandId : Payload : Purpose : Output")
	slog.Debug("---")
	// we loop through ASCII char that doesn't have special meanings
	for i := uint8(0x20); i < 0x7F; i++ {
		command := &Packet{PacketType: '3', CmdId: i, Payload: []byte{' '}, Timestamp: getTimestampNow()}
		response, err := l.executeAndRead(command)

		// not sure the purpose is related to firmware version, below is checked on FW 05.5.08.059_20230518
		var purpose string
		switch i {
		case 0x35: // '5'
			purpose = "firmware version"
		case 0x43: // 'C'
			purpose = "device serial number"
		case 0x4c: // 'L'
			purpose = "ambient light reporting enabled"
		case 0x4e: // 'N'
			purpose = "v-sync event enabled"
		case 0x55: // 'U'
			purpose = "magnetometer enabled"
		case 0x65: // 'e'
			purpose = "whether activated"
		case 0x66: // 'f'
			purpose = "activation time"
		case 0x68: // 'h'
			purpose = "RGB camera enabled"
		case 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2e:
			purpose = "n/a"
		default:
			purpose = "unknown"
		}
		if err != nil {
			slog.Error(fmt.Sprintf("('3' : '0x%x (%c)' : ' ' : %s : '%s') failed: %v", i, i, purpose, string(response), err))
			continue
		}
		slog.Info(fmt.Sprintf("'3' : '0x%x (%c)' : ' ' : %s : '%s'", i, i, purpose, string(response)))
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
