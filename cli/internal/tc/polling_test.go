package tc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var pollT0 = time.Date(2026, 9, 30, 20, 0, 0, 0, time.UTC)

type fakePollingAPI struct {
	name       string
	mu         *sync.Mutex
	writes     *[]string
	enabled    bool
	lastCallAt *time.Time
	missing    bool
	ignoreSet  bool
}

func (f *fakePollingAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.missing || r.URL.Path != pollingPath {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if r.Method == http.MethodPut {
		var req pollingSetRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		*f.writes = append(*f.writes, f.name+"="+map[bool]string{true: "on", false: "off"}[req.Enabled])
		if !f.ignoreSet {
			f.enabled = req.Enabled
		}
	}
	_ = json.NewEncoder(w).Encode(pollingStatus{Enabled: f.enabled, Source: "set", LastCallAt: f.lastCallAt})
}

type pollingRig struct {
	dev, prod *fakePollingAPI
	cfgs      map[string]Config
	writes    []string
	now       time.Time
	slept     []time.Duration
}

func newPollingRig(t *testing.T) *pollingRig {
	t.Helper()
	r := &pollingRig{now: pollT0}
	mu := &sync.Mutex{}
	r.dev = &fakePollingAPI{name: "dev", mu: mu, writes: &r.writes}
	r.prod = &fakePollingAPI{name: "prod", mu: mu, writes: &r.writes}
	devSrv, prodSrv := httptest.NewServer(r.dev), httptest.NewServer(r.prod)
	t.Cleanup(devSrv.Close)
	t.Cleanup(prodSrv.Close)
	r.cfgs = map[string]Config{"dev": {URL: devSrv.URL, APIKey: "d"}, "prod": {URL: prodSrv.URL, APIKey: "p"}}
	return r
}

func (r *pollingRig) clock() pollingClock {
	return pollingClock{
		now: func() time.Time { return r.now },
		sleep: func(_ context.Context, d time.Duration) error {
			r.slept = append(r.slept, d)
			r.now = r.now.Add(d)
			return nil
		},
	}
}

func (r *pollingRig) run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	env, out, errBuf := captureEnv()
	code := runPollingSwitch(context.Background(), r.cfgs, env, ParseArgs(append([]string{"polling-switch"}, args...)), r.clock())
	return code, out.String(), errBuf.String()
}

func ago(d time.Duration) *time.Time {
	t := pollT0.Add(-d)
	return &t
}

func TestPollingSwitch_ToDevTurnsProdOffFirstAndWaitsOutInFlight(t *testing.T) {
	t.Parallel()
	r := newPollingRig(t)
	r.prod.enabled, r.prod.lastCallAt = true, ago(10*time.Second)

	code, out, errOut := r.run(t, "--to", "dev", "--reason", "dev trial")

	if code != exitOK {
		t.Fatalf("code = %d stderr = %s", code, errOut)
	}
	if strings.Join(r.writes, ",") != "prod=off,dev=on" {
		t.Fatalf("writes = %v, want prod off before dev on", r.writes)
	}
	if len(r.slept) != 1 || r.slept[0] != 2*time.Minute {
		t.Fatalf("slept = %v, want 2m (in-flight limit plus gap)", r.slept)
	}
	if !strings.Contains(out, "Before:") || !strings.Contains(out, "After:") {
		t.Fatalf("out = %s", out)
	}
}

func TestPollingSwitch_ToProdWithDevAlreadyOffWaitsOnlyForLastCall(t *testing.T) {
	t.Parallel()
	r := newPollingRig(t)
	r.dev.lastCallAt = ago(30 * time.Second)

	code, _, errOut := r.run(t, "--to", "prod")

	if code != exitOK || strings.Join(r.writes, ",") != "prod=on" {
		t.Fatalf("code = %d writes = %v stderr = %s", code, r.writes, errOut)
	}
	if len(r.slept) != 1 || r.slept[0] != 30*time.Second {
		t.Fatalf("slept = %v, want 30s", r.slept)
	}
}

func TestPollingSwitch_NoWaitWhenOtherIsQuiet(t *testing.T) {
	t.Parallel()
	r := newPollingRig(t)
	r.dev.lastCallAt = ago(time.Hour)

	if code, _, _ := r.run(t, "--to", "prod"); code != exitOK || len(r.slept) != 0 {
		t.Fatalf("code = %d slept = %v", code, r.slept)
	}
}

func TestPollingSwitch_OffTurnsBothOff(t *testing.T) {
	t.Parallel()
	r := newPollingRig(t)
	r.dev.enabled, r.prod.enabled = true, true

	code, _, _ := r.run(t, "--to", "off")

	if code != exitOK || strings.Join(r.writes, ",") != "dev=off,prod=off" || len(r.slept) != 0 {
		t.Fatalf("code = %d writes = %v slept = %v", code, r.writes, r.slept)
	}
}

func TestPollingSwitch_FailedReadBackNeverTurnsTargetOn(t *testing.T) {
	t.Parallel()
	r := newPollingRig(t)
	r.prod.enabled, r.prod.ignoreSet = true, true

	code, _, errOut := r.run(t, "--to", "dev")

	if code != exitRuntime || strings.Join(r.writes, ",") != "prod=off" {
		t.Fatalf("code = %d writes = %v", code, r.writes)
	}
	if !strings.Contains(errOut, "Failed to turn prod off") || !strings.Contains(errOut, "Current state:") {
		t.Fatalf("stderr = %s", errOut)
	}
}

func TestPollingSwitch_MissingOtherEnvironment(t *testing.T) {
	t.Parallel()
	r := newPollingRig(t)
	r.prod.missing = true

	code, _, errOut := r.run(t, "--to", "dev")
	if code != exitRuntime || len(r.writes) != 0 || !strings.Contains(errOut, "--allow-missing true") {
		t.Fatalf("code = %d writes = %v stderr = %s", code, r.writes, errOut)
	}

	code, _, errOut = r.run(t, "--to", "dev", "--allow-missing", "true")
	if code != exitOK || strings.Join(r.writes, ",") != "dev=on" {
		t.Fatalf("allow-missing: code = %d writes = %v stderr = %s", code, r.writes, errOut)
	}
}

func TestPollingSwitch_MissingTargetAlwaysFails(t *testing.T) {
	t.Parallel()
	r := newPollingRig(t)
	r.prod.missing = true

	if code, _, _ := r.run(t, "--to", "prod", "--allow-missing", "true"); code != exitRuntime || len(r.writes) != 0 {
		t.Fatalf("code = %d writes = %v", code, r.writes)
	}
}

func TestPollingSwitch_RejectsBadTarget(t *testing.T) {
	t.Parallel()
	r := newPollingRig(t)
	for _, args := range [][]string{{}, {"--to", "staging"}} {
		if code, _, _ := r.run(t, args...); code != exitUsage {
			t.Errorf("%v: code = %d", args, code)
		}
	}
	if len(r.writes) != 0 {
		t.Fatalf("writes = %v", r.writes)
	}
}

func TestPollingStatus_ShowsBothEnvironments(t *testing.T) {
	t.Parallel()
	r := newPollingRig(t)
	r.prod.enabled, r.prod.lastCallAt = true, ago(time.Minute)
	env, out, _ := captureEnv()

	code := runPollingStatus(context.Background(), r.cfgs, env)

	if code != exitOK {
		t.Fatalf("code = %d", code)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[1], "dev   off") || !strings.HasPrefix(lines[2], "prod  on") || !strings.Contains(lines[2], "2026-09-30 19:59:00") {
		t.Fatalf("out =\n%s", out.String())
	}
}

func TestLoadEnvironments(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	good := filepath.Join(dir, "good.json")
	_ = os.WriteFile(good, []byte(`{"url":"u","apiKey":"k","environments":{"dev":{"url":"d","apiKey":"dk"},"prod":{"url":"p","apiKey":"pk"}}}`), 0o600)
	missing := filepath.Join(dir, "missing.json")
	_ = os.WriteFile(missing, []byte(`{"environments":{"prod":{"url":"p","apiKey":"pk"}}}`), 0o600)

	cfgs, err := LoadEnvironments(good)
	if err != nil || cfgs["dev"] != (Config{URL: "d", APIKey: "dk"}) || cfgs["prod"] != (Config{URL: "p", APIKey: "pk"}) {
		t.Fatalf("cfgs = %v err = %v", cfgs, err)
	}
	if _, err := LoadEnvironments(missing); err == nil || !strings.Contains(err.Error(), "environments.dev.url") {
		t.Fatalf("err = %v", err)
	}
}
