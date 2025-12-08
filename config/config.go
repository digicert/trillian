package config

import (
	"github.com/digicert/ctutils/logging"
	"github.com/digicert/ctutils/logging/adapters"
)

// InitLogging sets up the logging adapter for the project.
func InitLogging() {
	// Initialize OpenTelemetry with config struct
	logging.InitOpenTelemetry(logging.TelemetryConfigFromEnv())

	// Example: select logger backend via config, env var, or flag
	logCfg := logging.Config{Format: "json"}
	adapter := adapters.NewLogrusAdapter(logCfg)
	logging.SetLoggerAdapter(adapter)
}
