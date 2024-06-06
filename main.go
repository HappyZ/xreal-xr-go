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
	} else if _, ok := constant.SupportedFirmwareVersion[light.Name()][firmware]; !config.SkipFirmwareCheck && !ok {
		slog.Error(fmt.Sprintf("your device has a firmware that is not validated: validated ones include %v", constant.SupportedFirmwareVersion))

		if !confirmToContinue() {
			return
		}
	}

	line := liner.NewLiner()
	defer line.Close()

	line.SetCtrlCAborts(true)

	for {
		input, err := line.Prompt(">> ")
		if err != nil {
			if err == liner.ErrPromptAborted {
				slog.Warn("aborted")
				return
			}
			if err.Error() == "EOF" && input == "" {
				slog.Warn("EOF, exited")
				return
			}
			slog.Error(fmt.Sprintf("error reading input: %v", err))
			return
		}

		input = strings.TrimSpace(input)
		if input != "" {
			line.AppendHistory(input)
		}

		switch {
		case strings.HasPrefix(input, "get"):
			handleGetCommand(light, input)
		case strings.HasPrefix(input, "set"):
			handleSetCommand(light, input)
		case strings.HasPrefix(input, "test"):
			handleDevTestCommand(light, input)
		default:
			if input == "" {
				continue
			}
			if (input == "exit") || (input == "quit") || (input == "stop") || (input == "q") {
				return
			}
			slog.Error("unknown command")
		}
	}
}

func handleGetCommand(d device.Device, input string) {
	parts := strings.Split(input, " ")
	if len(parts) != 2 {
		slog.Error(fmt.Sprintf("invalid command format: get len(%v)=%d. Use 'get <command>'", parts, len(parts)))
		return
	}

	command := parts[1]

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
