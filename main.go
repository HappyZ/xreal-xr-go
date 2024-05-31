package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"strings"

	"xreal-light-xr-go/constant"
	"xreal-light-xr-go/device"
)

func parseFlags() constant.Config {
	var config constant.Config

	flag.BoolVar(&config.Debug, "debug", false, "if set to true, enable verbose logging output")
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

	light := device.NewXREALLight(nil, nil)

	err := light.Connect()
	if err != nil {
		slog.Error(fmt.Sprintf("failed to connect: %v", err))
		return
	}
	defer light.Disconnect()

	if firmware, err := light.GetFirmwareVersion(); err != nil {
		slog.Error(fmt.Sprintf("failed to get firmware version from device: %v", err))
		return
	} else if _, ok := constant.SupportedFirmwareVersion[firmware]; !config.SkipFirmwareCheck && !ok {
		slog.Error(fmt.Sprintf("your device has a firmware that is not validated: validated ones include %v", constant.SupportedFirmwareVersion))

		fmt.Println("Do you still want to continue? (y/N) ")

		var input string
		fmt.Scanln(&input)

		if input != "y" && input != "Y" && input != "Yes" && input != "YES" {
			return
		}
	}

	for {
		fmt.Println(">> ")

		var input string
		fmt.Scanln(&input)

		switch {
		case strings.HasPrefix(input, "exit"), strings.HasPrefix(input, "quit"), strings.HasPrefix(input, "stop"):
			return
		case strings.HasPrefix(input, "get"):
			handleGetCommand(light, input)
		case strings.HasPrefix(input, "set"):
			handleSetCommand(light, input)
		default:
		}
	}
}

func handleGetCommand(light device.Device, input string) {
	parts := strings.Split(input, " ")
	if len(parts) != 2 {
		slog.Error("invalid command format. Use 'get <command>'")
		return
	}

	command := parts[1]

	switch command {
	case "serial":
		serial, err := light.GetSerial()
		if err != nil {
			slog.Error(fmt.Sprintf("failed to get serial: %v", err))
			return
		}
		slog.Info(fmt.Sprintf("Serial: %s", serial))
	case "displaymode":
		mode, err := light.GetDisplayMode()
		if err != nil {
			slog.Error(fmt.Sprintf("failed to get display mode: %v", err))
			return
		}
		slog.Info(fmt.Sprintf("Display Mode: %s", mode))
	default:
		slog.Error("unknown command")
	}
}

func handleSetCommand(light device.Device, input string) {
	parts := strings.Split(input, " ")
	if len(parts) < 3 {
		slog.Error("invalid command format. Use 'set <command> <args>'")
		return
	}

	command := parts[1]
	args := parts[2:]

	switch command {
	case "displaymode":
		if _, ok := device.SupportedDisplayMode[args[0]]; !ok {
			slog.Error(fmt.Sprintf("invalid display mode: got (%s) want one of (%v)", args[0], device.SupportedDisplayMode))
			return
		}
		err := light.SetDisplayMode(device.DisplayMode(args[0]))
		if err != nil {
			slog.Error(fmt.Sprintf("failed to set display mode: %v", err))
			return
		}
		slog.Info("Display mode set successfully")
	default:
		slog.Error("unknown command")
	}
}
