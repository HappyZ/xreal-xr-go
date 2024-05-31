package main

import (
	"fmt"

	"xreal-light-xr-go/device"
)

// TODO(happyz): Uses the following as tests right now
func main() {
	device := device.NewXREALLight(nil, nil)

	err := device.Connect()
	if err != nil {
		fmt.Printf("failed to connect: %v\n", err)
		return
	}
	defer device.Disconnect()

	serial, err := device.GetSerial()
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("serial: ", serial)

	mode, err := device.GetDisplayMode()
	if err != nil {
		fmt.Printf("failed to get display mode: %v\n", err)
		return
	}
	fmt.Printf("mode: %s\n", mode.String())
}
