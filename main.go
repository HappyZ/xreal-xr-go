package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"strings"

	"xreal-light-xr-go/constant"
	"xreal-light-xr-go/device"

	"github.com/peterh/liner"
)

func parseFlags() constant.Config {
	var config constant.Config

	flag.BoolVar(&config.ConnectAtStart, "connect-at-start", false, "if set, connect glass right away")
	flag.BoolVar(&config.Debug, "debug", false, "if set, enable verbose logging output")
	flag.BoolVar(&config.SkipFirmwareCheck, "skip-firmware-check", false, "if set to true, we do not validate firmware")

	flag.Parse()

	return config
}

func main() {
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

	if config.ConnectAtStart {
		glassDevice = handleDeviceConnection("connect any", config)
		if glassDevice == nil {
			slog.Warn("device not connected")
		}
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
			glassDevice = handleDeviceConnection(input, config)
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
					slog.Info(fmt.Sprintf("- path: %s - serialNumber: %s", info.Path, info.SerialNbr))
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

func handleDeviceConnection(input string, config constant.Config) device.Device {
	parts := strings.Split(input, " ")
	if len(parts) != 2 {
		slog.Error(fmt.Sprintf("invalid command format: connect len(%v)=%d. Use 'connect <any/serial>'", parts, len(parts)))
		return nil
	}

	var glassDevice device.Device
	switch parts[1] {
	case "any":
		glassDevice = device.NewXREALLight(nil, nil)
	default:
		// assume it's serial number
		glassDevice = device.NewXREALLight(nil, &parts[1])
	}

	err := glassDevice.Connect()
	if err != nil {
		slog.Error(fmt.Sprintf("failed to connect: %v", err))
		return nil
	}

	if firmware, err := glassDevice.GetFirmwareVersion(); err != nil {
		slog.Error(fmt.Sprintf("failed to get firmware version from device: %v", err))
		glassDevice.Disconnect()
		return nil
	} else if _, ok := constant.SupportedFirmwareVersion[glassDevice.Name()][firmware]; !config.SkipFirmwareCheck && !ok {
		slog.Error(fmt.Sprintf("your device has a firmware that is not validated: validated ones include %v", constant.SupportedFirmwareVersion))

		if !confirmToContinue() {
			glassDevice.Disconnect()
			return nil
		}
	}
	return glassDevice
}

func handleGetCommand(d device.Device, input string) {
	parts := strings.Split(input, " ")
	if len(parts) < 2 {
		slog.Error(fmt.Sprintf("invalid command format: get len(%v)=%d. Use 'get <command> <optional:args>'", parts, len(parts)))
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
	case "options":
		results := d.GetOptionsEnabled(args)
		for i, result := range results {
			slog.Info(fmt.Sprintf("%s: %s", args[i], result))
		}
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
		err := d.SetBrightnessLevel(args[0])
		if err != nil {
			slog.Error(fmt.Sprintf("failed to set brightness level: %v", err))
			return
		}
		slog.Info("Display mode set successfully")
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
	if len(parts) < 2 {
		slog.Error(fmt.Sprintf("invalid command format: get len(%v)=%d. Use 'test <command> <optional:args>'", parts, len(parts)))
		return
	}

	command := parts[1]

	switch command {
	default:
		if len(command) == 1 { // single char input
			if confirmToContinue() {
				d.DevExecuteAndRead(parts[1:])
			}
			return
		}
		slog.Error("unknown command")
	}
}
