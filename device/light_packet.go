package device

import (
	"bytes"
	"fmt"
	"log/slog"
	"strconv"
	"time"
	"xreal-light-xr-go/crc"
)

var DUMMY_PAYLOAD = []byte{' '}

type Packet struct {
	Type      PacketType
	Command   *Command
	Payload   []byte
	Timestamp []byte
	Message   string
}

// PacketType tells the type of the decoded Packet for Light communications
type PacketType int

const (
	PACKET_TYPE_UNKNOWN PacketType = iota
	PACKET_TYPE_CRC_ERROR
	PACKET_TYPE_COMMAND
	PACKET_TYPE_RESPONSE
	PACKET_TYPE_MCU
	PACKET_TYPE_HEART_BEAT_RESPONSE
)

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
		if pkt.Command.ID == 0x4b || pkt.Command.ID == 0x4c || pkt.Command.ID == 0x4d || pkt.Command.ID == 0x50 || pkt.Command.ID == 0x53 {
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

func newCommandPacket(cmd *Command, payload ...[]byte) *Packet {
	defaultPayload := DUMMY_PAYLOAD
	if len(payload) > 0 {
		defaultPayload = payload[0]
	}
	return &Packet{
		Type:      PACKET_TYPE_COMMAND,
		Command:   cmd,
		Payload:   defaultPayload,
		Timestamp: getTimestampNow(),
	}
}

func getTimestampNow() []byte {
	return []byte(fmt.Sprintf("%x", (time.Now().UnixMilli())))
}
