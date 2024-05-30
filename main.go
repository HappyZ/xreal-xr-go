package main

import (
	"fmt"

	"xreal-light-xr-go/device"
)

func main() {
	device, err := device.NewXREALLight(nil)
	if err != nil {
		fmt.Printf("failed to create new device: %v\n", err)
		return
	}
	mode, err := device.GetDisplayMode()
	if err != nil {
		fmt.Printf("failed to get display mode: %v\n", err)
		return
	}
	fmt.Printf("mode: %v\n", mode)
}
