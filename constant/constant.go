package constant

const (
	XREAL_LIGHT = "XREAL Light"
)

// Config holds configuration options for xrealxr
type Config struct {
	// Enable verbose logging output
	Debug bool
	// Do not validate firmware
	SkipFirmwareCheck bool
}

var SupportedFirmwareVersion = map[string]map[string]struct{}{
	XREAL_LIGHT: {"05.5.08.059_20230518": {}},
}
