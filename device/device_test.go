package device_test

import (
	"reflect"
	"testing"

	"xreal-light-xr-go/device"
)

func TestSerializeDeserializeCommandSuccessfully(t *testing.T) {
	testCases := []struct {
		command       *device.Command
		expectedBytes []byte
		expectedError error
	}{
		{
			command: &device.Command{
				CmdType:   uint8('a'),
				CmdId:     uint8('b'),
				Payload:   []byte{'c', 'd'},
				Timestamp: uint8('e'),
			},
		},
	}

	for _, tc := range testCases {
		serialized, err := tc.command.Serialize()

		if err != nil {
			t.Errorf("serialize error: %v", err)
			return
		}

		newCommand := &device.Command{}
		err = newCommand.Deserialize(serialized)
		if err != nil {
			t.Errorf("deserialize error: %v", err)
			return
		}

		if !reflect.DeepEqual(tc.command, newCommand) {
			t.Errorf("expected: %v, got: %v", tc.command, newCommand)
		}
	}
}
