package device_test

import (
	"fmt"
	"reflect"
	"testing"

	"xreal-light-xr-go/device"
)

func TestSerializeDeserializeCommandSuccessfully(t *testing.T) {
	testCases := []struct {
		command       *device.Packet
		expectedBytes []byte
		expectedError error
	}{
		{
			command: &device.Packet{
				PacketType: uint8('a'),
				CmdId:      uint8('b'),
				Payload:    []byte{'c', 'd'},
				Timestamp:  []byte{'e', 'f'},
			},
		},
	}

	for _, tc := range testCases {
		serialized, err := tc.command.Serialize()

		if err != nil {
			t.Errorf("serialize error: %v", err)
			return
		}

		fmt.Printf("serialized: %v\n", serialized)

		deserialized := &device.Packet{}
		err = deserialized.Deserialize(serialized[:])
		if err != nil {
			t.Errorf("deserialize error: %v", err)
			return
		}

		fmt.Printf("deserialized: %v\n", deserialized)

		if !reflect.DeepEqual(tc.command, deserialized) {
			t.Errorf("expected: %v, got: %v", tc.command, deserialized)
		}
	}
}
