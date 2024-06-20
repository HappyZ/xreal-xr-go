package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"xreal-light-xr-go/constant"
	"xreal-light-xr-go/device"

	"github.com/peterh/liner"
)

func parseFlags() constant.Config {
	var config constant.Config

	flag.BoolVar(&config.AutoConnect, "auto", false, "if set, connect the first attached glass automatically")
	flag.BoolVar(&config.Debug, "debug", false, "if set, enable debug logging output")

	flag.Parse()

	return config
}

func main() {
	// Following mainly used for debugging/development purposes.
	// Intention is to build an interface to build against and never need to use interactive command lines.

	config := parseFlags()

	log.SetFlags(log.Ldate | log.Lmicroseconds)
	if config.Debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	slog.Debug(fmt.Sprintf("config: %+v", config))

	var glassDevice device.Device

	defer func() {
		if glassDevice != nil {
			glassDevice.Disconnect()
		}
	}()

	if config.AutoConnect {
		glassDevice = waitAndConnectGlass()
	}

	line := liner.NewLiner()
	defer line.Close()

	line.SetCtrlCAborts(true)

	for {
		input, err := line.Prompt(">> ")
		if err != nil {
			if err == liner.ErrPromptAborted {
				continue
			}
			if err.Error() == "EOF" && input == "" {
				slog.Info("exiting..")
				return
			}
			slog.Error(fmt.Sprintf("error reading input: %v", err))
			return
		}

		input = strings.TrimSpace(input)
		if input != "" {
			line.AppendHistory(input)
		} else {
			continue
		}

		switch {
		case strings.HasPrefix(input, "connect"):
			glassDevice = handleDeviceConnection(input)
			if glassDevice == nil {
				slog.Warn("device not connected")
			}
		case strings.HasPrefix(input, "get"):
			if glassDevice == nil {
				slog.Error("device not connected, run connect first")
				continue
			}
			handleGetCommand(glassDevice, input)
		case strings.HasPrefix(input, "set"):
			if glassDevice == nil {
				slog.Error("device not connected, run connect first")
				continue
			}
			handleSetCommand(glassDevice, input)
		case strings.HasPrefix(input, "test"):
			if glassDevice == nil {
				slog.Error("device not connected, run connect first")
				continue
			}
			handleDevTestCommand(glassDevice, input)
		default:
			if input == "list" {
				devices, err := device.EnumerateDevices(0, 0)
				if err != nil {
					slog.Error(fmt.Sprintf("failed to enumerate hid devices: %v\n", err))
					continue
				}
				for _, info := range devices {
					slog.Info(fmt.Sprintf("- path: %s - serialNumber: %s - vid: %d - pid: %d", info.Path, info.SerialNbr, info.VendorID, info.ProductID))
				}
				continue
			}
			if (input == "exit") || (input == "quit") || (input == "stop") || (input == "q") {
				return
			}
			slog.Error("unknown command")
		}
	}
}

func waitAndConnectGlass() device.Device {
	for {
		glassDevice := handleDeviceConnection("connect any")
		if glassDevice == nil {
			slog.Info("retry in 10s...")
			time.Sleep(10 * time.Second)
			continue
		}
		return glassDevice
	}
}

func handleDeviceConnection(input string) device.Device {
	parts := strings.Split(input, " ")
	if len(parts) != 2 {
		slog.Error(fmt.Sprintf("invalid command format: connect len(%v)=%d. Use 'connect <any>'", parts, len(parts)))
		return nil
	}

	var glassDevice device.Device
	switch parts[1] {
	case "any":
		glassDevice = device.NewXREALLight()
	default:
		return nil
	}

	err := glassDevice.Connect()
	if err != nil {
		slog.Error(fmt.Sprintf("failed to connect: %v", err))
		return nil
	}
	return glassDevice
}

func handleGetCommand(d device.Device, input string) {
	parts := strings.Split(input, " ")
	if len(parts) < 2 {
		slog.Error(fmt.Sprintf("invalid command format: get len(%v)=%d. Use 'get <command>'", parts, len(parts)))
		return
	}

	command := parts[1]
	args := parts[2:]

	switch command {
	case "serial":
		serial, err := d.GetSerial()
		if err != nil {
			slog.Error(fmt.Sprintf("failed to get serial: %v", err))
			return
		}
		slog.Info(fmt.Sprintf("Serial: %s", serial))
	case "displaymode":
		mode, err := d.GetDisplayMode()
		if err != nil {
			slog.Error(fmt.Sprintf("failed to get display mode: %v", err))
			return
		}
		slog.Info(fmt.Sprintf("Display Mode: %s", mode))
	case "brightness":
		brightness, err := d.GetBrightnessLevel()
		if err != nil {
			slog.Error(fmt.Sprintf("failed to get brightness level: %v", err))
			return
		}
		slog.Info(fmt.Sprintf("Brightness Level: %s", brightness))
	case "image", "images":
		if len(args) == 0 || !isDir(args[0]) {
			slog.Error(fmt.Sprintf("invalid input: %v", args))
			return
		}
		filepaths, err := d.GetImages(args[0])
		if err != nil {
			slog.Error(fmt.Sprintf("failed to dump images: %v", err))
			return
		}
		slog.Info(fmt.Sprintf("dumped to file location: %v", filepaths))
	default:
		slog.Error("unknown command")
	}
}

func handleSetCommand(d device.Device, input string) {
	parts := strings.Split(input, " ")
	if len(parts) < 2 {
		slog.Error(fmt.Sprintf("invalid command format: get len(%v)=%d. Use 'set <command> <optional:args>'", parts, len(parts)))
		return
	}

	command := parts[1]
	args := parts[2:]

	switch command {
	case "displaymode":
		if len(args) == 0 {
			slog.Error(fmt.Sprintf("empty display mode input, please specify one of (%v)", device.SupportedDisplayMode))
			return
		}
		if _, ok := device.SupportedDisplayMode[args[0]]; !ok {
			slog.Error(fmt.Sprintf("invalid display mode: got (%s) want one of (%v)", args[0], device.SupportedDisplayMode))
			return
		}
		err := d.SetDisplayMode(device.DisplayMode(args[0]))
		if err != nil {
			slog.Error(fmt.Sprintf("failed to set display mode: %v", err))
			return
		}
		slog.Info("Display mode set successfully")
	case "brightness":
		if len(args) == 0 {
			slog.Error("empty brightness level input, please specify a number")
			return
		}
		if err := d.SetBrightnessLevel(args[0]); err != nil {
			slog.Error(fmt.Sprintf("failed to set brightness level: %v", err))
			return
		}
		slog.Info("Display mode set successfully")
	case "vsync", "ambientlight", "magnetometer", "temperature", "imu", "rgbcam", "sleep":
		if len(args) == 0 || (args[0] != "0" && args[0] != "1") {
			slog.Error("empty input, please specify 0 (disable) or 1 (enable)")
			return
		}
		var err error
		switch command {
		case "vsync":
			err = d.EnableEventReporting(device.CMD_ENABLE_VSYNC, args[0])
		case "ambientlight":
			err = d.EnableEventReporting(device.CMD_ENABLE_AMBIENT_LIGHT, args[0])
		case "magnetometer":
			err = d.EnableEventReporting(device.CMD_ENABLE_MAGNETOMETER, args[0])
		case "temperature":
			err = d.EnableEventReporting(device.CMD_ENABLE_TEMPERATURE, args[0])
		case "rgbcam":
			err = d.EnableEventReporting(device.CMD_ENABLE_RGB_CAMERA, args[0])
		case "imu":
			err = d.EnableEventReporting(device.OV580_ENABLE_IMU_STREAM, args[0])
		case "sleep":
			err = d.EnableEventReporting(device.CMD_SET_SLEEP_TIME, args[0])
		}
		if err != nil {
			slog.Error(fmt.Sprintf("failed to set %s event: %v", command, err))
			return
		}
		slog.Info(fmt.Sprintf("%s event reporting set successfully", command))
	default:
		slog.Error("unknown command")
	}
}

func confirmToContinue() bool {
	line := liner.NewLiner()
	defer line.Close()

	line.SetCtrlCAborts(true)

	input, err := line.Prompt("Please confirm if you want to continue? (y/N) ")
	if err != nil {
		if err == liner.ErrPromptAborted {
			slog.Warn("aborted, taking it as a NO")
			return false
		}
		if err.Error() == "EOF" && input == "" {
			slog.Warn("EOF, taking it as a NO")
			return false
		}
		slog.Error(fmt.Sprintf("error reading input: %v", err))
		return false
	}

	input = strings.TrimSpace(input)

	if input != "y" && input != "Y" && input != "Yes" && input != "YES" {
		return false
	}
	return true
}

func handleDevTestCommand(d device.Device, input string) {
	parts := strings.Split(input, " ")
	if len(parts) < 3 {
		slog.Error(fmt.Sprintf("invalid command format: get len(%v)=%d. Use 'test mcu/ov580 <command> <optional:args>'", parts, len(parts)))
		return
	}

	device := parts[1]
	command := parts[2]
	args := parts[3:]

	switch device {
	case "mcu", "ov580":
		if len(command) == 1 { // single char input
			if confirmToContinue() {
				d.DevExecuteAndRead(device, parts[2:])
			}
			return
		}
		slog.Error("unknown command")
	case "camera":
		switch command {
		case "images":
			if len(args) == 0 {
				slog.Error("needs folder path")
				return
			}
			if filepaths, err := d.GetImagesDataDev(args[0]); err != nil {
				slog.Error(err.Error())
			} else {
				slog.Info(fmt.Sprintf("dumped to %v", filepaths))
			}
		default:
			slog.Error("unknown device")
		}
	default:
		slog.Error("unknown device")
	}
}

func isDir(path string) bool {
	// Use os.Stat to get file info
	info, err := os.Stat(path)
	if err != nil {
		// Handle error
		if os.IsNotExist(err) {
			return false // Path does not exist
		}
		return false // Other error, treat as not directory
	}

	// Check if the path is a directory
	return info.IsDir()
}
