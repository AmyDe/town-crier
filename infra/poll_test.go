package main

import (
	"strings"
	"testing"
)

func TestPollJobEnv_ProdSettings(t *testing.T) {
	t.Parallel()

	want := map[string]string{
		"PLANIT_BASE_URL":                     "https://www.planit.org.uk/",
		"POLLING_DAILY_CALL_CAP":              "240",
		"POLLING_MIN_REQUEST_SPACING_SECONDS": "60",
		"POLLING_DELTA_SLOTS":                 "09:00,12:00,15:00,17:00",
		"POLLING_DELTA_MAX_PAGES":             "20",
		"POLLING_DAY_ALLOWANCE":               "60",
		"POLLING_FULL_READ_MAX_AGE_DAYS":      "7",
		"POLLING_RUN_BUDGET_MINUTES":          "55",
		"NOTIFY_QUIET_START":                  "22:00",
		"NOTIFY_QUIET_END":                    "07:00",
		"NOTIFY_EVENT_SURGE_THRESHOLD":        "10000",
	}
	got := pollSettings("prod")
	if len(got) != len(want) {
		t.Errorf("got %d vars, want %d: %v", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
}

func TestPollJobEnv_DevSettings(t *testing.T) {
	t.Parallel()

	got := pollSettings("dev")
	want := map[string]string{
		"POLLING_AREA_ID":                devPollingAreaID,
		"POLLING_DAILY_CALL_CAP":         "60",
		"POLLING_ORACLE_ENABLED":         "true",
		"POLLING_FULL_READ_MAX_AGE_DAYS": devPollingFullReadMaxAgeDays,
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
}

func TestAddGoWorkerEnv_PollGetsPollEnvAndOthersDoNot(t *testing.T) {
	t.Parallel()

	if got := addGoWorkerEnv(nil, envContext{env: "prod"}, "poll"); len(got) <= len(pollJobEnv("prod")) {
		t.Errorf("poll worker env has %d vars, want more than the %d poll settings", len(got), len(pollJobEnv("prod")))
	}
	base := len(addGoWorkerEnv(nil, envContext{env: "prod"}, "pg-purge"))
	if got := len(addGoWorkerEnv(nil, envContext{env: "prod"}, "poll")); got <= base+len(pollJobEnv("prod")) {
		t.Errorf("poll env count %d does not include poll settings plus push env over base %d", got, base)
	}
}

func TestPlanitPollAlertSpecs_SeveritiesAndScope(t *testing.T) {
	t.Parallel()

	wantSev := map[string]float64{
		"alert-planit-poll-critical-shared":      1,
		"alert-planit-poll-degraded-shared":      2,
		"alert-planit-poll-heartbeat-shared":     2,
		"alert-planit-poll-critical-dev-shared":  3,
		"alert-planit-poll-degraded-dev-shared":  3,
		"alert-planit-poll-heartbeat-dev-shared": 3,
	}
	specs := planitPollAlertSpecs()
	if len(specs) != len(wantSev) {
		t.Fatalf("got %d specs, want %d", len(specs), len(wantSev))
	}
	for _, s := range specs {
		want, ok := wantSev[s.name]
		if !ok {
			t.Errorf("unexpected alert %q", s.name)
			continue
		}
		if s.severity != want {
			t.Errorf("%s severity = %v, want %v", s.name, s.severity, want)
		}
		if !strings.Contains(s.query, `Name == "PlanIt poll run"`) {
			t.Errorf("%s query does not target the poll run span", s.name)
		}
		env := "prod"
		if strings.Contains(s.name, "-dev-") {
			env = "dev"
		}
		if !strings.Contains(s.query, `deployment.environment"]) == "`+env+`"`) {
			t.Errorf("%s query is not scoped to %s", s.name, env)
		}
	}
}
