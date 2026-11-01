package admin

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
)

type fakeReadPathSwitch struct {
	st      applications.ReadPathState
	patches []applications.ReadPathPatch
}

func (f *fakeReadPathSwitch) State(context.Context) (applications.ReadPathState, error) {
	return f.st, nil
}

func (f *fakeReadPathSwitch) Set(_ context.Context, p applications.ReadPathPatch, reason string, at time.Time) (applications.ReadPathState, error) {
	f.patches = append(f.patches, p)
	if p.ThinSync != nil {
		f.st.ThinSync = *p.ThinSync
	}
	if p.ThinList != nil {
		f.st.ThinList = *p.ThinList
	}
	if p.ThinMap != nil {
		f.st.ThinMap = *p.ThinMap
	}
	f.st.Source, f.st.Reason, f.st.UpdatedAt = applications.ReadPathSourceSet, reason, &at
	return f.st, nil
}

func readPathRequest(t *testing.T, sw *fakeReadPathSwitch, method, key, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	mux := http.NewServeMux()
	ReadPathRoutes(mux, "k", sw, func() time.Time { return pollingNow }, slog.New(slog.DiscardHandler))
	req := httptest.NewRequestWithContext(context.Background(), method, "/v1/admin/read-path", strings.NewReader(body))
	if key != "" {
		req.Header.Set(adminKeyHeader, key)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var out map[string]any
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode %q: %v", rec.Body.String(), err)
		}
	}
	return rec, out
}

func TestReadPath_GetReportsEverySwitchOffByDefault(t *testing.T) {
	t.Parallel()
	sw := &fakeReadPathSwitch{st: applications.ReadPathState{Source: applications.ReadPathSourceDefault}}

	rec, out := readPathRequest(t, sw, http.MethodGet, "k", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	for _, k := range []string{"thinSync", "thinList", "thinMap"} {
		if out[k] != false {
			t.Errorf("%s = %v, want false", k, out[k])
		}
	}
	if out["source"] != "default" || out["updatedAt"] != nil {
		t.Errorf("out = %v", out)
	}
}

func TestReadPath_PutSetsEachSwitchAndReturnsStoredState(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"thinSync", "thinList", "thinMap"} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			sw := &fakeReadPathSwitch{}

			rec, out := readPathRequest(t, sw, http.MethodPut, "k", `{"`+key+`":true,"reason":"stage 2"}`)

			if rec.Code != http.StatusOK || len(sw.patches) != 1 {
				t.Fatalf("status = %d patches = %d", rec.Code, len(sw.patches))
			}
			for _, k := range []string{"thinSync", "thinList", "thinMap"} {
				if want := k == key; out[k] != want {
					t.Errorf("%s = %v, want %v", k, out[k], want)
				}
			}
			if out["source"] != "set" || out["reason"] != "stage 2" || out["updatedAt"] != "2026-09-30T20:00:00Z" {
				t.Errorf("out = %v", out)
			}
		})
	}
}

func TestReadPath_PutRejectsBadBody(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{"reason":"no switch"}`, `not json`, `{"thinSync":"yes"}`, `{}`} {
		sw := &fakeReadPathSwitch{}
		rec, _ := readPathRequest(t, sw, http.MethodPut, "k", body)
		if rec.Code != http.StatusBadRequest || len(sw.patches) != 0 {
			t.Errorf("%s: status = %d patches = %d", body, rec.Code, len(sw.patches))
		}
	}
}

func TestReadPath_RequiresAdminKey(t *testing.T) {
	t.Parallel()
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		for _, key := range []string{"", "wrong"} {
			sw := &fakeReadPathSwitch{}
			rec, _ := readPathRequest(t, sw, method, key, `{"thinSync":true}`)
			if rec.Code != http.StatusUnauthorized || len(sw.patches) != 0 {
				t.Errorf("%s key %q: status = %d patches = %d", method, key, rec.Code, len(sw.patches))
			}
		}
	}
}
