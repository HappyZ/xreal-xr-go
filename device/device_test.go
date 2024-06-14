package device_test

import (
	"fmt"
	"log/slog"
	"reflect"
	"testing"

	"xreal-light-xr-go/device"
)

func TestSerializeDeserializeCommandSuccessfully(t *testing.T) {
	testCases := []struct {
		packet *device.Packet
	}{
		{
			packet: &device.Packet{
				Type:      device.PACKET_TYPE_COMMAND,
				Command:   &device.CMD_GET_BRIGHTNESS_LEVEL,
				Payload:   device.DUMMY_PAYLOAD,
				Timestamp: []byte("18fd37a61db"), // epoch: 1717239964 (seconds) 123 (milliseconds)
			},
		},
	}

	for _, tc := range testCases {
		serialized, err := tc.packet.Serialize()

		if err != nil {
			t.Errorf("serialize error: %v", err)
			return
		}

		slog.Info(fmt.Sprintf("serialized: %v\n", serialized))

		deserialized := &device.Packet{}
		err = deserialized.Deserialize(serialized[:])
		if err != nil {
			t.Errorf("deserialize error: %v", err)
			return
		}

		slog.Info(fmt.Sprintf("deserialized: %v\n", deserialized))

		if !reflect.DeepEqual(tc.packet, deserialized) {
			t.Errorf("expected: %v, got: %v", tc.packet, deserialized)
		}
	}
}
