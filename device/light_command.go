package device

import (
	"xreal-light-xr-go/constant"
)

type CommandInstruction int

const (
	CMD_UKNOWN CommandInstruction = iota

	CMD_GET_BRIGHTNESS_LEVEL
	CMD_SET_BRIGHTNESS_LEVEL

	CMD_GET_DISPLAY_HDCP
	CMD_GET_DISPLAY_MODE
	CMD_SET_DISPLAY_MODE

	CMD_GET_AMBIENT_LIGHT_ENABLED
	CMD_ENABLE_AMBIENT_LIGHT
	CMD_GET_MAGNETOMETER_ENABLED
	CMD_ENABLE_MAGNETOMETER
	CMD_GET_VSYNC_ENABLED
	CMD_ENABLE_VSYNC
	CMD_GET_TEMPERATURE_ENABLED
	CMD_ENABLE_TEMPERATURE
	CMD_ENABLE_RGB_CAMERA

	CMD_GET_GLASS_ACTIVATED
	CMD_SET_GLASS_ACTIVATION
	CMD_GET_GLASS_ACTIVATION_TIME

	CMD_HEART_BEAT
	CMD_GET_NREAL_FW_STRING
	CMD_GET_FIRMWARE_VERSION
	CMD_GET_DISPLAY_FIRMWARE
	CMD_GET_SERIAL_NUMBER
	CMD_GET_STOCK_FIRMWARE_VERSION
	CMD_SET_MAX_BRIGHTNESS_LEVEL
	CMD_SET_SDK_WORKS

	MCU_EVENT_AMBIENT_LIGHT
	MCU_EVENT_KEY_PRESS
	MCU_EVENT_MAGNETOMETER
	MCU_EVENT_PROXIMITY
	MCU_EVENT_TEMPERATURE_A
	MCU_EVENT_TEMPERATURE_B
	MCU_EVENT_VSYNC

	OV580_ENABLE_IMU_STREAM
	OV580_GET_CALIBRATION_FILE_LENGTH
	OV580_GET_CALIBRATION_FILE_PART
)

type Command struct {
	Type uint8
	ID   uint8

	// This is a hidden, optional field
	instruction CommandInstruction
}

func (cmd Command) Equals(another *Command) bool {
	if another == nil {
		return false
	}
	return (cmd.Type == another.Type) && (cmd.ID == another.ID)
}

func (cmd Command) EqualsInstruction(instruction CommandInstruction) bool {
	if cmd.instruction == CMD_UKNOWN {
		foundCommand := GetFirmwareIndependentCommand(instruction)
		return cmd.Equals(foundCommand)
	}
	return cmd.instruction == instruction
}

func (cmd Command) String() string {
	switch cmd.instruction {
	case CMD_GET_STOCK_FIRMWARE_VERSION:
		return "get stock firmware version"
	case CMD_SET_BRIGHTNESS_LEVEL:
		return "set brightness level"
	case CMD_GET_BRIGHTNESS_LEVEL:
		return "get brightness level"
	case CMD_SET_MAX_BRIGHTNESS_LEVEL:
		return "set max brightness level"
	case CMD_SET_DISPLAY_MODE:
		return "set display mode"
	case CMD_GET_DISPLAY_MODE:
		return "get display mode"
	case CMD_GET_DISPLAY_FIRMWARE:
		return "get display firmware version"
	case CMD_GET_FIRMWARE_VERSION:
		return "get firmware version"
	case CMD_GET_SERIAL_NUMBER:
		return "get glass serial number"
	case CMD_HEART_BEAT:
		return "send heart beat"
	case CMD_ENABLE_AMBIENT_LIGHT:
		return "enable ambient light reporting"
	case CMD_GET_AMBIENT_LIGHT_ENABLED:
		return "get if ambient light reporting enabled"
	case CMD_ENABLE_VSYNC:
		return "eanble v-sync reporting"
	case CMD_GET_VSYNC_ENABLED:
		return "get if v-sync reporting enabled"
	case CMD_ENABLE_MAGNETOMETER:
		return "enable geo magnetometer reporting"
	case CMD_GET_MAGNETOMETER_ENABLED:
		return "get if geo magnetometer reporting enabled"
	case CMD_ENABLE_TEMPERATURE:
		return "enable temperature reporting"
	case CMD_ENABLE_RGB_CAMERA:
		return "enable RGB camera"
	case CMD_GET_TEMPERATURE_ENABLED:
		return "get if temperature reporting enabled"
	case CMD_SET_GLASS_ACTIVATION:
		return "set glass activation"
	case CMD_GET_GLASS_ACTIVATED:
		return "get if glass activated"
	case CMD_GET_GLASS_ACTIVATION_TIME:
		return "get glass activation time (epoch, sec)"
	case CMD_GET_NREAL_FW_STRING:
		return "always returns hardcoded string `NrealFW`"
	case CMD_SET_SDK_WORKS:
		return "set or unset SDK works"
	case MCU_EVENT_AMBIENT_LIGHT:
		return "ambient light report event"
	case MCU_EVENT_KEY_PRESS:
		return "key pressed report event"
	case MCU_EVENT_MAGNETOMETER:
		return "magnetometer report event"
	case MCU_EVENT_PROXIMITY:
		return "proximity report event"
	case MCU_EVENT_TEMPERATURE_A, MCU_EVENT_TEMPERATURE_B:
		return "temperature report event"
	case MCU_EVENT_VSYNC:
		return "v-sync report event"
	case OV580_ENABLE_IMU_STREAM:
		return "(ov580) enable IMU sensor stream reporting"
	case OV580_GET_CALIBRATION_FILE_LENGTH:
		return "(ov580) get calibration file length before reading it"
	case OV580_GET_CALIBRATION_FILE_PART:
		return "(ov580) read the calibration file part"
	default:
		return "unknown / no function"
	}
}

func GetFirmwareIndependentCommand(instruction CommandInstruction) *Command {
	var command *Command

	switch instruction {
	case CMD_GET_NREAL_FW_STRING: // hardcoded string `NrealFW`
		command = &Command{Type: 0x33, ID: 0x56}
	case CMD_HEART_BEAT:
		command = &Command{Type: 0x40, ID: 0x4b}
	case CMD_GET_FIRMWARE_VERSION: // this must be firmware independent
		// another option is Command{Type: 0x33, ID: 0x61}, so far the same
		command = &Command{Type: 0x33, ID: 0x35}
	case CMD_GET_DISPLAY_MODE:
		command = &Command{Type: 0x33, ID: 0x33}
	case CMD_SET_DISPLAY_MODE:
		command = &Command{Type: 0x31, ID: 0x33}
	case CMD_GET_AMBIENT_LIGHT_ENABLED:
		command = &Command{Type: 0x33, ID: 0x4c}
	case CMD_ENABLE_AMBIENT_LIGHT:
		command = &Command{Type: 0x31, ID: 0x4c}
	case CMD_GET_VSYNC_ENABLED:
		command = &Command{Type: 0x33, ID: 0x4e}
	case CMD_ENABLE_VSYNC:
		command = &Command{Type: 0x31, ID: 0x4e}
	case CMD_GET_MAGNETOMETER_ENABLED:
		command = &Command{Type: 0x33, ID: 0x55}
	case CMD_ENABLE_MAGNETOMETER:
		command = &Command{Type: 0x31, ID: 0x55}
	case CMD_GET_TEMPERATURE_ENABLED:
		command = &Command{Type: 0x33, ID: 0x60}
	case CMD_ENABLE_TEMPERATURE:
		command = &Command{Type: 0x31, ID: 0x60}
	case CMD_GET_GLASS_ACTIVATED:
		command = &Command{Type: 0x33, ID: 0x65}
	case CMD_SET_GLASS_ACTIVATION:
		command = &Command{Type: 0x31, ID: 0x65}
	case CMD_GET_GLASS_ACTIVATION_TIME:
		command = &Command{Type: 0x33, ID: 0x66}
	case CMD_ENABLE_RGB_CAMERA:
		command = &Command{Type: 0x31, ID: 0x68}
	case CMD_GET_BRIGHTNESS_LEVEL:
		command = &Command{Type: 0x33, ID: 0x31}
	case CMD_SET_BRIGHTNESS_LEVEL:
		// another option is Command{Type: 0x31, ID: 0x59}, but upon testing it doesn't do what's expected in newer firmware, see https://github.com/badicsalex/ar-drivers-rs/issues/14#issuecomment-2148616976
		command = &Command{Type: 0x31, ID: 0x31}
	case CMD_GET_SERIAL_NUMBER:
		command = &Command{Type: 0x33, ID: 0x43}
	case CMD_GET_STOCK_FIRMWARE_VERSION:
		command = &Command{Type: 0x33, ID: 0x30}
	case CMD_SET_SDK_WORKS:
		command = &Command{Type: 0x40, ID: 0x33}
	case MCU_EVENT_AMBIENT_LIGHT:
		command = &Command{Type: 0x35, ID: 0x4c}
	case MCU_EVENT_KEY_PRESS:
		command = &Command{Type: 0x35, ID: 0x4b}
	case MCU_EVENT_MAGNETOMETER:
		command = &Command{Type: 0x35, ID: 0x4d}
	case MCU_EVENT_PROXIMITY:
		command = &Command{Type: 0x35, ID: 0x50}
	case MCU_EVENT_TEMPERATURE_A: // needs further investigations
		command = &Command{Type: 0x35, ID: 0x52}
	case MCU_EVENT_TEMPERATURE_B: // needs further investigations
		command = &Command{Type: 0x35, ID: 0x54}
	case MCU_EVENT_VSYNC:
		command = &Command{Type: 0x35, ID: 0x53}
	case OV580_ENABLE_IMU_STREAM:
		command = &Command{Type: 0x02, ID: 0x19}
	case OV580_GET_CALIBRATION_FILE_LENGTH:
		command = &Command{Type: 0x02, ID: 0x14}
	case OV580_GET_CALIBRATION_FILE_PART: // only parts returned so need to run multiple times
		command = &Command{Type: 0x02, ID: 0x15}
	default:
	}

	if command != nil {
		command.instruction = instruction
	}

	return command
}

func (l *xrealLightMCU) getCommand(instruction CommandInstruction) *Command {
	var command *Command

	command = GetFirmwareIndependentCommand(instruction)
	if command != nil {
		return command
	}

	// the following is known to be firmware dependent
	firmwareVersion := l.glassFirmware
	switch instruction {
	case CMD_GET_DISPLAY_HDCP: // hardcoded "ELLA2_1224_HDCP"
		switch firmwareVersion {
		case constant.FIRMWARE_05_5_08_059:
			command = &Command{Type: 0x33, ID: 0x48}
		case constant.FIRMWARE_05_1_08_021:
			command = &Command{Type: 0x33, ID: 0x34}
		default:
		}
	case CMD_SET_MAX_BRIGHTNESS_LEVEL: // shouldn't do anything, static, does not take any input
		switch firmwareVersion {
		case constant.FIRMWARE_05_5_08_059:
			command = &Command{Type: 0x31, ID: 0x32}
		case constant.FIRMWARE_05_1_08_021:
			command = &Command{Type: 0x33, ID: 0x32}
		default:
		}
	case CMD_GET_DISPLAY_FIRMWARE:
		switch firmwareVersion {
		case constant.FIRMWARE_05_5_08_059: // "ELLA2_0518_V017"
			command = &Command{Type: 0x33, ID: 0x34}
		default:
		}
	default:
	}

	if command != nil {
		command.instruction = instruction
	}
	return command
}

// var (
// 	// FIRMWARE_05_1_08_021 only
// 	// CMD_SET_MAX_BRIGHTNESS_LEVEL     = Command{Type: 0x33, ID: 0x32} // shouldn't do anything, static, does not take any input
// 	// CMD_GET_DISPLAY_HDCP             = Command{Type: 0x33, ID: 0x34} // hardcoded "ELLA2_1224_HDCP"
// 	// FIRMWARE_05_1_08_021 and above
// 	CMD_GET_STOCK_FIRMWARE_VERSION   = Command{Type: 0x33, ID: 0x30}
// 	CMD_SET_BRIGHTNESS_LEVEL_0       = Command{Type: 0x31, ID: 0x31}
// 	CMD_GET_BRIGHTNESS_LEVEL         = Command{Type: 0x33, ID: 0x31}
// 	CMD_SET_DISPLAY_MODE             = Command{Type: 0x31, ID: 0x33}
// 	CMD_GET_DISPLAY_MODE             = Command{Type: 0x33, ID: 0x33}
// 	CMD_GET_DISPLAY_MODE_STRING      = Command{Type: 0x33, ID: 0x64} // not very useful given CMD_GET_DISPLAY_MODE
// 	CMD_GET_FIRMWARE_VERSION_0       = Command{Type: 0x33, ID: 0x35}
// 	CMD_GET_FIRMWARE_VERSION_1       = Command{Type: 0x33, ID: 0x61} // same as CMD_GET_FIRMWARE_VERSION_0
// 	CMD_SET_POWER                    = Command{Type: 0x31, ID: 0x39} // unknown purpose, input '0'/'1'
// 	CMD_GET_POWER                    = Command{Type: 0x33, ID: 0x39} // unknown purpose, default to '0'
// 	CMD_CLEAR_EEPROM_VALUE           = Command{Type: 0x31, ID: 0x41} // untested, input 4 byte eeprom address, set to 0xff
// 	CMD_GET_SERIAL_NUMBER            = Command{Type: 0x33, ID: 0x43}
// 	CMD_SET_APPROACH_PS_VALUE        = Command{Type: 0x31, ID: 0x44} // unknown purpose, input integer string
// 	CMD_GET_APPROACH_PS_VALUE        = Command{Type: 0x33, ID: 0x44} // unknown purpose, mine by default is 130
// 	CMD_SET_DISTANCE_PS_VALUE        = Command{Type: 0x31, ID: 0x45} // unknown purpose, input integer string
// 	CMD_GET_DISTANCE_PS_VALUE        = Command{Type: 0x33, ID: 0x45} // unknown purpose, mine by default is 110
// 	CMD_GET_DISPLAY_VERSION          = Command{Type: 0x33, ID: 0x46} // unknown purpose, mine by default is ELLA2_07.20
// 	CMD_GET_DISPLAY_DEBUG_DATA       = Command{Type: 0x33, ID: 0x6b} // unknown purpose
// 	CMD_SET_EEPROM_0X27_SOMETHING    = Command{Type: 0x31, ID: 0x47} // untested
// 	CMD_GET_EEPROM_0X27_SOMETHING    = Command{Type: 0x33, ID: 0x47} // untested
// 	CMD_GET_EEPROM_0X43_SOMETHING    = Command{Type: 0x33, ID: 0x48} // untested
// 	CMD_SET_EEPROM_0X43_SOMETHING    = Command{Type: 0x40, ID: 0x41} // untested
// 	CMD_SET_EEPROM_0X95_SOMETHING    = Command{Type: 0x31, ID: 0x50} // untested
// 	CMD_REBOOT_GLASS                 = Command{Type: 0x31, ID: 0x52}
// 	CMD_SET_EEPROM_0X110_SOMETHING   = Command{Type: 0x40, ID: 0x53} // untested
// 	CMD_GET_EEPROM_ADDR_VALUE        = Command{Type: 0x33, ID: 0x4b}
// 	CMD_GET_ORBIT_FUNC               = Command{Type: 0x33, ID: 0x37} // unknown purpose
// 	CMD_SET_ORBIT_FUNC               = Command{Type: 0x40, ID: 0x34} // input 0x0b (open) or others (close)
// 	CMD_SET_OLED_LEFT_HORIZONTAL     = Command{Type: 0x31, ID: 0x48} // unknown purpose, input is integer 0-255
// 	CMD_SET_OLED_LEFT_VERTICAL       = Command{Type: 0x31, ID: 0x49} // unknown purpose, input is integer 0-255
// 	CMD_SET_OLED_RIGHT_HORIZONTAL    = Command{Type: 0x31, ID: 0x4a} // unknown purpose, input is integer 0-255
// 	CMD_SET_OLED_RIGHT_VERTICAL      = Command{Type: 0x31, ID: 0x4b} // unknown purpose, input is integer 0-255
// 	CMD_GET_OLED_LRHV_VALUE          = Command{Type: 0x33, ID: 0x4a} // unknown purpose, LH-LV-RH-RV values set above, mine default with 'L05L06R27R26'
// 	MCU_KEY_PRESS                    = Command{Type: 0x35, ID: 0x4b}
// 	CMD_ENABLE_AMBIENT_LIGHT         = Command{Type: 0x31, ID: 0x4c}
// 	CMD_GET_AMBIENT_LIGHT_ENABLED    = Command{Type: 0x33, ID: 0x4c}
// 	MCU_AMBIENT_LIGHT                = Command{Type: 0x35, ID: 0x4c}
// 	CMD_SET_DUTY                     = Command{Type: 0x31, ID: 0x4d} // affect display brightness, input is integer 0-100
// 	CMD_GET_DUTY                     = Command{Type: 0x33, ID: 0x4d}
// 	CMD_ENABLE_VSYNC                 = Command{Type: 0x31, ID: 0x4e} // input '0'/'1'
// 	CMD_GET_ENABLE_VSYNC_ENABLED     = Command{Type: 0x33, ID: 0x4e} // mine default with 1
// 	MCU_VSYNC                        = Command{Type: 0x35, ID: 0x53}
// 	MCU_PROXIMITY                    = Command{Type: 0x35, ID: 0x50}
// 	CMD_SET_SLEEP_TIME               = Command{Type: 0x31, ID: 0x51} // input is integer that's larger than 20
// 	CMD_GET_SLEEP_TIME               = Command{Type: 0x33, ID: 0x51} // mine by default is 60
// 	CMD_GET_GLASS_START_UP_NUM       = Command{Type: 0x33, ID: 0x52} // unknown purpose
// 	CMD_GET_GLASS_ERROR_NUM          = Command{Type: 0x54, ID: 0x46} // unknown purpose
// 	CMD_GLASS_SLEEP                  = Command{Type: 0x54, ID: 0x47}
// 	CMD_GET_SOME_VALUE               = Command{Type: 0x33, ID: 0x53} // unknown purpose, output a digit
// 	CMD_RESET_OV580                  = Command{Type: 0x31, ID: 0x54} // untested
// 	CMD_ENABLE_MAGNETOMETER          = Command{Type: 0x31, ID: 0x55} // input '0'/'1'
// 	CMD_GET_MAGNETOMETER_ENABLED     = Command{Type: 0x33, ID: 0x55}
// 	CMD_READ_MAGNETOMETER            = Command{Type: 0x54, ID: 0x45} // untested
// 	MCU_MAGNETOMETER                 = Command{Type: 0x35, ID: 0x4d}
// 	CMD_GET_NREAL_FW_STRING          = Command{Type: 0x33, ID: 0x56} // hardcoded string `NrealFW`
// 	CMD_GET_MCU_SERIES               = Command{Type: 0x33, ID: 0x58} // hardcoded string `STM32F413MGY6`
// 	CMD_GET_MCU_ROM_SIZE             = Command{Type: 0x33, ID: 0x59} // hardcoded string `ROM_1.5Mbytes`
// 	CMD_GET_MCU_RAM_SIZE             = Command{Type: 0x33, ID: 0x5a} // hardcoded string `RAM_320Kbytes`
// 	CMD_UPDATE_DISPLAY_FW_UPDATE     = Command{Type: 0x31, ID: 0x58} // dont do this to light, it bricks my dev glasses
// 	CMD_SET_BRIGHTNESS_LEVEL_1       =
// 	CMD_ENABLE_TEMPERATURE           = Command{Type: 0x31, ID: 0x60} // untested, input '0'/'1'
// 	CMD_GET_TEMPERATURE_ENABLED      = Command{Type: 0x33, ID: 0x60} // untested, guessed
// 	CMD_SET_OLED_BRIGHTNESS_LEVEL    = Command{Type: 0x31, ID: 0x62} // untested, input '0'/'1'
// 	CMD_GET_OLED_BRIGHTNESS_LEVEL    = Command{Type: 0x33, ID: 0x62} // untested
// 	CMD_SET_ACTIVATION               = Command{Type: 0x31, ID: 0x65} // untested, input '0'/'1'
// 	CMD_GET_ACTIVATION               = Command{Type: 0x33, ID: 0x65}
// 	CMD_SET_ACTIVATION_TIME          = Command{Type: 0x31, ID: 0x66} // untested, input 8 bytes (up to epoch seconds)
// 	CMD_GET_ACTIVATION_TIME          = Command{Type: 0x33, ID: 0x66}
// 	CMD_SET_SUPER_ACTIVE             = Command{Type: 0x31, ID: 0x67} // unknown, input '0'/'1'
// 	CMD_ENABLE_RGB_CAMERA            = Command{Type: 0x31, ID: 0x68} // untested, input '0'/'1'
// 	CMD_POWER_OFF_RGB_CAMERA         = Command{Type: 0x54, ID: 0x56} // untested
// 	CMD_POWER_ON_RGB_CAMERA          = Command{Type: 0x54, ID: 0x57} // untested
// 	CMD_GET_RGB_CAMERA_ENABLED       = Command{Type: 0x33, ID: 0x68}
// 	CMD_ENABLE_STEREO_CAMERA         = Command{Type: 0x31, ID: 0x69} // untested, input '0'/'1', OV580
// 	CMD_GET_STEREO_CAMERA_ENABLED    = Command{Type: 0x33, ID: 0x69}
// 	CMD_SET_DEBUG_LOG                = Command{Type: 0x40, ID: 0x31} // untested, input 0x08 (Usart) / 0x07 (CRC) / 0 disable both
// 	CMD_CHECK_SONY_OTP_STUFF         = Command{Type: 0x40, ID: 0x32} // untested
// 	CMD_SET_SDK_WORKS                = Command{Type: 0x40, ID: 0x33} // input '0'/'1'
// 	CMD_MCU_B_JUMP_TO_A              = Command{Type: 0x40, ID: 0x38} // untested, for firmware update
// 	CMD_MCU_UPDATE_FW_ON_A_START     = Command{Type: 0x40, ID: 0x39} // untested, for firmware update
// 	CMD_DEFAULT_2D_FUNC_ENABLE       = Command{Type: 0x40, ID: 0x46} // unknown
// 	CMD_KEYSWITCH_ENABLE             = Command{Type: 0x40, ID: 0x48} // unknown
// 	CMD_HEART_BEAT                   = Command{Type: 0x40, ID: 0x4b}
// 	CMD_UPDATE_DISPLAY_SUCCESS       = Command{Type: 0x40, ID: 0x4d} // this doesn't do much
// 	CMD_MCU_A_JUMP_TO_B              = Command{Type: 0x40, ID: 0x52} // untested, for firmware update
// 	CMD_DATA_KEY_SOMETHING           = Command{Type: 0x40, ID: 0x52} // unknown, input '1'-'6' does different things
// 	CMD_SET_LIGHT_COMPENSATION       = Command{Type: 0x46, ID: 0x47} // untested
// 	CMD_CALIBRATE_LIGHT_COMPENSATION = Command{Type: 0x54, ID: 0x51} // untested
// 	CMD_RETRY_GET_OTP                = Command{Type: 0x54, ID: 0x52} // untested
// 	CMD_GET_OLED_BRIGHTNESS_BRIT     = Command{Type: 0x54, ID: 0x55} // untested
// 	// FIRMWARE_05_5_08_059 only
// 	CMD_SET_MAX_BRIGHTNESS_LEVEL = Command{Type: 0x31, ID: 0x32} // shouldn't do anything, static, does not take any input
// 	CMD_GET_DISPLAY_FIRMWARE     = Command{Type: 0x33, ID: 0x34} // "ELLA2_0518_V017"
// 	// CMD_GET_DISPLAY_HDCP         = Command{Type: 0x33, ID: 0x48} // "ELLA2_1224_HDCP"
// )
