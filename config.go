package trillian

import (
	"github.com/digicert/ctutils/logging"
	"github.com/digicert/ctutils/logging/adapters"
)

// InitLogging sets up the logging adapter for the project.
func InitLogging() {
	// Example: select logger backend via config, env var, or flag
	logConfig := logging.Config{Level: logging.InfoLevel, Format: "json"}
	adapter := adapters.NewLogrusAdapter(logConfig)
	logging.SetLoggerAdapter(adapter)
}
