package platform

import (
	"log/slog"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name      string
		port      string
		logLevel  string
		wantPort  string
		wantLevel slog.Level
		wantErr   bool
	}{
		{"defaults", "", "", "8080", slog.LevelInfo, false},
		{"port override", "9090", "", "9090", slog.LevelInfo, false},
		{"debug level", "", "debug", "8080", slog.LevelDebug, false},
		{"warn level", "", "WARN", "8080", slog.LevelWarn, false},
		{"invalid level", "", "noisy", "", slog.LevelInfo, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PORT", tc.port)
			t.Setenv("LOG_LEVEL", tc.logLevel)

			cfg, err := LoadConfig()

			if (err != nil) != tc.wantErr {
				t.Fatalf("got err=%v, wantErr=%v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if cfg.Port != tc.wantPort {
				t.Errorf("Port: got %q, want %q", cfg.Port, tc.wantPort)
			}
			if cfg.LogLevel != tc.wantLevel {
				t.Errorf("LogLevel: got %v, want %v", cfg.LogLevel, tc.wantLevel)
			}
		})
	}
}
