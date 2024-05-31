package main

import (
	"bufio"
	"fmt"
	"os"

	"xreal-light-xr-go/device"
)

// TODO(happyz): Uses the following as tests right now
func main() {
	light := device.NewXREALLight(nil, nil)

	err := light.Connect()
	if err != nil {
		fmt.Printf("failed to connect: %v\n", err)
		return
	}
	defer light.Disconnect()

	serial, err := light.GetSerial()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("serial: ", serial)

	firmware, err := light.GetFirmwareVersion()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("firmware: ", firmware)

	mode, err := light.GetDisplayMode()
	if err != nil {
		fmt.Printf("failed to get display mode: %v\n", err)
	}
	fmt.Printf("mode: %s\n", mode.String())

	err = light.SetDisplayMode(device.DISPLAY_MODE_STEREO)
	if err != nil {
		fmt.Printf("failed to set display mode: %v\n", err)
	} else {
		fmt.Printf("mode has set to: %s\n", device.DISPLAY_MODE_STEREO.String())
	}

	fmt.Println("Press enter to stop...")
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')
}
