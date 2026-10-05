package platform

import (
	"testing"
)

func TestLoadConfig_PollRebuildDefaults(t *testing.T) {
	for _, k := range []string{
		"PLANIT_BASE_URL", "POLLING_DAILY_CALL_CAP", "POLLING_MIN_REQUEST_SPACING_SECONDS",
		"POLLING_DELTA_SLOTS", "POLLING_DELTA_MAX_PAGES", "POLLING_DAY_ALLOWANCE",
		"POLLING_FULL_READ_MAX_AGE_DAYS", "POLLING_RUN_BUDGET_MINUTES", "POLLING_AREA_ID",
		"POLLING_ORACLE_ENABLED", "NOTIFY_QUIET_START", "NOTIFY_QUIET_END", "NOTIFY_EVENT_SURGE_THRESHOLD",
	} {
		t.Setenv(k, "")
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.PlanItBaseURL != "https://www.planit.org.uk/" {
		t.Errorf("PlanItBaseURL = %q", cfg.PlanItBaseURL)
	}
	if cfg.PollingDailyCallCap != 240 {
		t.Errorf("PollingDailyCallCap = %d, want 240", cfg.PollingDailyCallCap)
	}
	if cfg.PollingMinRequestSpacingSeconds != 60 {
		t.Errorf("PollingMinRequestSpacingSeconds = %d, want 60", cfg.PollingMinRequestSpacingSeconds)
	}
	if cfg.PollingDeltaSlots != "09:00,12:00,15:00,17:00" {
		t.Errorf("PollingDeltaSlots = %q", cfg.PollingDeltaSlots)
	}
	if cfg.PollingDeltaMaxPages != 20 {
		t.Errorf("PollingDeltaMaxPages = %d, want 20", cfg.PollingDeltaMaxPages)
	}
	if cfg.PollingDayAllowance != 60 {
		t.Errorf("PollingDayAllowance = %d, want 60", cfg.PollingDayAllowance)
	}
	if cfg.PollingFullReadMaxAgeDays != 7 {
		t.Errorf("PollingFullReadMaxAgeDays = %d, want 7", cfg.PollingFullReadMaxAgeDays)
	}
	if cfg.PollingRunBudgetMinutes != 55 {
		t.Errorf("PollingRunBudgetMinutes = %d, want 55", cfg.PollingRunBudgetMinutes)
	}
	if cfg.PollingAreaID != 0 {
		t.Errorf("PollingAreaID = %d, want 0 (all authorities)", cfg.PollingAreaID)
	}
	if cfg.PollingOracleEnabled {
		t.Error("PollingOracleEnabled = true, want false by default")
	}
	if cfg.NotifyQuietStart != "22:00" || cfg.NotifyQuietEnd != "07:00" {
		t.Errorf("quiet hours = %q-%q, want 22:00-07:00", cfg.NotifyQuietStart, cfg.NotifyQuietEnd)
	}
	if cfg.NotifyEventSurgeThreshold != 10000 {
		t.Errorf("NotifyEventSurgeThreshold = %d, want 10000", cfg.NotifyEventSurgeThreshold)
	}
}

func TestLoadConfig_PollRebuildOverrides(t *testing.T) {
	t.Setenv("POLLING_DAILY_CALL_CAP", "60")
	t.Setenv("POLLING_MIN_REQUEST_SPACING_SECONDS", "30")
	t.Setenv("POLLING_DELTA_SLOTS", "10:00,14:00")
	t.Setenv("POLLING_DELTA_MAX_PAGES", "5")
	t.Setenv("POLLING_DAY_ALLOWANCE", "10")
	t.Setenv("POLLING_FULL_READ_MAX_AGE_DAYS", "0")
	t.Setenv("POLLING_RUN_BUDGET_MINUTES", "30")
	t.Setenv("POLLING_AREA_ID", "314")
	t.Setenv("POLLING_ORACLE_ENABLED", "true")
	t.Setenv("NOTIFY_QUIET_START", "23:00")
	t.Setenv("NOTIFY_QUIET_END", "06:30")
	t.Setenv("NOTIFY_EVENT_SURGE_THRESHOLD", "500")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.PollingDailyCallCap != 60 || cfg.PollingMinRequestSpacingSeconds != 30 ||
		cfg.PollingDeltaSlots != "10:00,14:00" || cfg.PollingDeltaMaxPages != 5 ||
		cfg.PollingDayAllowance != 10 || cfg.PollingFullReadMaxAgeDays != 0 ||
		cfg.PollingRunBudgetMinutes != 30 || cfg.PollingAreaID != 314 || !cfg.PollingOracleEnabled ||
		cfg.NotifyQuietStart != "23:00" || cfg.NotifyQuietEnd != "06:30" || cfg.NotifyEventSurgeThreshold != 500 {
		t.Errorf("overrides not applied: %+v", cfg)
	}
}

func TestLoadConfig_PollingAreaIDRejectsInvalid(t *testing.T) {
	for _, raw := range []string{"kingston", "-1", "1.5"} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv("POLLING_AREA_ID", raw)
			if _, err := LoadConfig(); err == nil {
				t.Fatalf("LoadConfig with POLLING_AREA_ID=%q: want error", raw)
			}
		})
	}
}
