package constant

const (
	XREAL_LIGHT          = "XREAL Light"
	XREAL_AIR            = "XREAL Air"
	XREAL_AIR_2          = "XREAL Air 2"
	XREAL_AIR_2_PRO      = "XREAL Air 2 Pro"
	XREAL_AIR_2_ULTRA    = "XREAL Air 2 Ultra"
	FIRMWARE_05_1_08_021 = "05.1.08.021_20221114"
	FIRMWARE_05_5_08_059 = "05.5.08.059_20230518"
)

// Config holds configuration options for xrealxr
type Config struct {
	// Enables debug logging output
	Debug bool
	// Immediately tries connect to a glass device at start
	AutoConnect bool
}
