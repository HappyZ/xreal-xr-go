package crc_test

import (
	"testing"

	"xreal-light-xr-go/crc"
)

func TestCRC32(t *testing.T) {
	testCases := []struct {
		input    []byte
		expected uint32
	}{
		{[]byte("Hello, world!"), 0xebe6c6e6},
		{[]byte("Lorem ipsum dolor sit amet"), 0x5f29d461},
		// Add more test cases as needed
	}

	for _, tc := range testCases {
		actual := crc.CRC32(tc.input)
		if actual != tc.expected {
			t.Errorf("CRC32(%q) = %08X; expected %08X", tc.input, actual, tc.expected)
		}
	}
}
