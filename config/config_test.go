package config

import (
	"os"
	"testing"

	"github.com/digicert/ctutils/logging"
)

func TestInitLogging(t *testing.T) {
	// dedicated test cases can use t.Setenv in Go 1.17+
	// For now using os.Setenv with defer cleanup for compatibility
	
	tests := []struct {
		name      string
		logFormat string
		logLevel  string
	}{
		{"Default", "", ""},
		{"JSON_Debug", "json", "DEBUG"},
		{"Text_Error", "text", "ERROR"},
		{"Invalid", "invalid", "INVALID"}, // Should fallback to defaults
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.logFormat != "" {
				os.Setenv("LOG_FORMAT", tt.logFormat)
				defer os.Unsetenv("LOG_FORMAT")
			}
			if tt.logLevel != "" {
				os.Setenv("LOG_LEVEL", tt.logLevel)
				defer os.Unsetenv("LOG_LEVEL")
			}

			// Ideally we would inspect the set logger, but since the global logger
			// is hidden/private in ctutils or hard to inspect without getters,
			// we at least verify this doesn't panic.
			InitLogging()

			// Check that a logger is set (not nil)
			if logging.GetLogger() == nil {
				t.Error("InitLogging() failed to set global logger")
			}
		})
	}
}
