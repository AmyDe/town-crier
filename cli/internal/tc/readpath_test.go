package tc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeReadPathAPI struct {
	state    readPathStatus
	puts     []map[string]any
	notFound bool
	gotKey   string
}

func (f *fakeReadPathAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.gotKey = r.Header.Get(apiKeyHeader)
	if f.notFound || r.URL.Path != readPathPath {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if r.Method == http.MethodPut {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.puts = append(f.puts, body)
		if v, ok := body["thinSync"].(bool); ok {
			f.state.ThinSync = v
		}
		if v, ok := body["thinList"].(bool); ok {
			f.state.ThinList = v
		}
		if v, ok := body["thinMap"].(bool); ok {
			f.state.ThinMap = v
		}
		f.state.Source = "set"
	}
	_ = json.NewEncoder(w).Encode(f.state)
}

func runReadPath(t *testing.T, api *fakeReadPathAPI, args ...string) (int, string, string) {
	t.Helper()
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)
	cfgs := map[string]Config{"dev": {URL: srv.URL, APIKey: "dev-key"}, "prod": {URL: "http://127.0.0.1:1", APIKey: "p"}}
	env, out, errBuf := captureEnv()
	code := runReadPathSwitch(context.Background(), cfgs, env, ParseArgs(append([]string{"read-path-switch"}, args...)))
	return code, out.String(), errBuf.String()
}

func TestReadPathSwitch_SetsEachSwitch(t *testing.T) {
	t.Parallel()
	for flag, field := range map[string]string{"thin-sync": "thinSync", "thin-list": "thinList", "thin-map": "thinMap"} {
		for value, want := range map[string]bool{"on": true, "off": false} {
			t.Run(flag+"="+value, func(t *testing.T) {
				t.Parallel()
				api := &fakeReadPathAPI{state: readPathStatus{Source: "default"}}

				code, out, errOut := runReadPath(t, api, "--env", "dev", "--"+flag, value, "--reason", "test")

				if code != exitOK {
					t.Fatalf("code = %d stderr = %s", code, errOut)
				}
				if len(api.puts) != 1 || len(api.puts[0]) != 2 || api.puts[0][field] != want || api.puts[0]["reason"] != "test" {
					t.Fatalf("puts = %v, want only %s=%v and reason", api.puts, field, want)
				}
				if api.gotKey != "dev-key" {
					t.Errorf("admin key = %q", api.gotKey)
				}
				if !strings.Contains(out, "THIN_SYNC") {
					t.Errorf("out = %s, want the switch table", out)
				}
			})
		}
	}
}

func TestReadPathSwitch_WithoutSwitchFlagsOnlyReads(t *testing.T) {
	t.Parallel()
	api := &fakeReadPathAPI{state: readPathStatus{ThinSync: true, Source: "set"}}

	code, out, _ := runReadPath(t, api, "--env", "dev")

	if code != exitOK || len(api.puts) != 0 || !strings.Contains(out, "on") {
		t.Errorf("code = %d puts = %v out = %s", code, api.puts, out)
	}
}

func TestReadPathSwitch_RejectsBadUsage(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{},
		{"--env", "staging"},
		{"--env", "dev", "--thin-sync", "maybe"},
		{"--thin-sync", "on"},
	} {
		api := &fakeReadPathAPI{}
		code, _, errOut := runReadPath(t, api, args...)
		if code != exitUsage || len(api.puts) != 0 || !strings.Contains(errOut, "Usage") {
			t.Errorf("%v: code = %d puts = %v stderr = %s", args, code, api.puts, errOut)
		}
	}
}

func TestReadPathSwitch_ReportsMissingRoute(t *testing.T) {
	t.Parallel()
	code, _, errOut := runReadPath(t, &fakeReadPathAPI{notFound: true}, "--env", "dev", "--thin-sync", "on")
	if code != exitRuntime || !strings.Contains(errOut, "older API") {
		t.Errorf("code = %d stderr = %s", code, errOut)
	}
}

func TestReadPathSwitch_FailsWhenReadBackDiffers(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(readPathStatus{Source: "set"})
	}))
	t.Cleanup(srv.Close)
	cfgs := map[string]Config{"dev": {URL: srv.URL, APIKey: "k"}, "prod": {URL: srv.URL, APIKey: "k"}}
	env, _, errBuf := captureEnv()

	code := runReadPathSwitch(context.Background(), cfgs, env, ParseArgs([]string{"read-path-switch", "--env", "dev", "--thin-sync", "on"}))

	if code != exitRuntime || !strings.Contains(errBuf.String(), "read back") {
		t.Errorf("code = %d stderr = %s", code, errBuf.String())
	}
}
