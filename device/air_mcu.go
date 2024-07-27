package device

/*
#cgo CFLAGS: -g -Wall
#cgo pkg-config: libusb-1.0 hidapi-libusb
#include <hidapi/hidapi.h>
#include <libusb-1.0/libusb.h>
#include <stdio.h>
#include <stdlib.h>

*/
import "C"
import (
	"fmt"
	"sync"
)

const (
	XREAL_AIR_SERIES_MCU_VID = uint16(0x3318)
	XREAL_AIR_MCU_PID        = uint16(0x0424)
	XREAL_AIR_2_MCU_PID      = uint16(0x0428)
	XREAL_AIR_2_PRO_MCU_PID  = uint16(0x0432)
	//TODO(happyz): Adds Ultra PID here
)

type xrealAirMCU struct {
	initialized bool

	// deviceHandlers contains callback funcs for the events from the glass device
	deviceHandlers *DeviceHandlers

	// glassFirmware is obtained from mcuDevice and used to get the correct commands
	glassFirmware string

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

func (a *xrealAirMCU) connectAndInitialize(vid uint16, pid uint16) error {

	// test cgo
	if err := C.hid_init(); err != 0 {
		return fmt.Errorf("failed to initialize hidapi")
	}
	defer C.hid_exit()

	handle := C.hid_open(C.ushort(vid), C.ushort(pid), nil)
	if handle == nil {
		return fmt.Errorf("failed to open glass MCU")
	}
	defer C.hid_close(handle)

	return nil
}
