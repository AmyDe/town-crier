package main

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

func pollTestConfig() platform.Config {
	return platform.Config{
		PlanItBaseURL:                   "https://stub.planit.test/",
		PollingDailyCallCap:             240,
		PollingHourlyCallCap:            15,
		PollingMinRequestSpacingSeconds: 60,
		PollingDeltaSlots:               "09:00,12:00,15:00,17:00",
		PollingDeltaMaxPages:            20,
		PollingDayAllowance:             60,
		PollingFullReadMaxAgeDays:       7,
		PollingRunBudgetMinutes:         55,
		NotifyQuietStart:                "22:00",
		NotifyQuietEnd:                  "07:00",
		NotifyEventSurgeThreshold:       10000,
	}
}

func lazyPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), "postgres://u:p@127.0.0.1:1/none?sslmode=disable")
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestBuildPoller_WiresWithAndWithoutAreaAndOracle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*platform.Config)
	}{
		{"prod: all areas, no oracle", func(*platform.Config) {}},
		{"dev: one area, oracle on", func(c *platform.Config) {
			c.PollingAreaID = 314
			c.PollingOracleEnabled = true
			c.PollingDailyCallCap = 60
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := pollTestConfig()
			tc.mutate(&cfg)

			poller, err := buildPoller(cfg, lazyPool(t), testRegistry(), &stores{lease: nil}, discardLogger())
			if err != nil {
				t.Fatalf("buildPoller: %v", err)
			}
			if poller == nil {
				t.Fatal("buildPoller: nil poller")
			}
		})
	}
}

func TestBuildPoller_RejectsInvalidSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*platform.Config)
	}{
		{"bad delta slot", func(c *platform.Config) { c.PollingDeltaSlots = "9am" }},
		{"bad quiet start", func(c *platform.Config) { c.NotifyQuietStart = "late" }},
		{"bad quiet end", func(c *platform.Config) { c.NotifyQuietEnd = "" }},
		{"missing PlanIt base URL", func(c *platform.Config) { c.PlanItBaseURL = "" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := pollTestConfig()
			tc.mutate(&cfg)

			if _, err := buildPoller(cfg, lazyPool(t), testRegistry(), &stores{}, discardLogger()); err == nil {
				t.Fatal("buildPoller: want error")
			}
		})
	}
}
