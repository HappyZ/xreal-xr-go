package constant

const (
	XREAL_LIGHT          = "XREAL Light"
	FIRMWARE_05_1_08_021 = "05.1.08.021_20221114"
	FIRMWARE_05_5_08_059 = "05.5.08.059_20230518"
)

// Config holds configuration options for xrealxr
type Config struct {
	// Enable verbose logging output
	Debug bool
	// Do not validate firmware
	SkipFirmwareCheck bool
}

var SupportedFirmwareVersion = map[string]map[string]struct{}{
	XREAL_LIGHT: {FIRMWARE_05_5_08_059: {}},
}
