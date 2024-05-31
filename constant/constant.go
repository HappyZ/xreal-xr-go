package constant

var (
	SupportedFirmwareVersion = map[string]struct{}{
		"05.5.08.059_20230518": {}, // XREAL firmware from Nebula APK 3.8.0
	}
)

// Config holds configuration options for xrealxr
type Config struct {
	// Enable verbose logging output
	Debug bool
	// Do not validate firmware
	SkipFirmwareCheck bool
}
