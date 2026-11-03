package admin

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/erasure"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

type fakeErasureFinder struct {
	byEmail map[string]string
	err     error
	asked   []string
}

func (f *fakeErasureFinder) GetByEmail(_ context.Context, email string) (*profiles.UserProfile, error) {
	f.asked = append(f.asked, email)
	if f.err != nil {
		return nil, f.err
	}
	id, ok := f.byEmail[email]
	if !ok {
		return nil, profiles.ErrNotFound
	}
	return &profiles.UserProfile{UserID: id, Email: &email}, nil
}

type fakeEraseStep struct {
	name  string
	calls *[]string
	err   error
}

func (s fakeEraseStep) record(userID string) error {
	*s.calls = append(*s.calls, s.name+":"+userID)
	return s.err
}

func (s fakeEraseStep) DeleteAllByUserID(_ context.Context, userID string) error {
	return s.record(userID)
}

func (s fakeEraseStep) AnonymiseRedemptionsByUserID(_ context.Context, userID string) error {
	return s.record(userID)
}

func (s fakeEraseStep) Delete(_ context.Context, userID string) error { return s.record(userID) }

func (s fakeEraseStep) DeleteUser(_ context.Context, userID string) error { return s.record(userID) }

func newFakeDeleters(calls *[]string, auth0Err error) erasure.Deleters {
	step := func(name string) fakeEraseStep { return fakeEraseStep{name: name, calls: calls} }
	return erasure.Deleters{
		Notifications:       step("notifications"),
		WatchZones:          step("watchzones"),
		SavedApplications:   step("saved"),
		DeviceRegistrations: step("devices"),
		NotificationState:   step("state"),
		OfferCodes:          step("offercodes"),
		Profile:             step("profile"),
		Auth0:               fakeEraseStep{name: "auth0", calls: calls, err: auth0Err},
	}
}

func eraseRequest(t *testing.T, finder *fakeErasureFinder, deleters erasure.Deleters, key, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	ErasureRoutes(mux, "k", finder, deleters, slog.New(slog.DiscardHandler))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/admin/users/erase", strings.NewReader(body))
	if key != "" {
		req.Header.Set(adminKeyHeader, key)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestErasure_ErasesEveryStepForTheUserFoundByEmail(t *testing.T) {
	t.Parallel()
	var calls []string
	finder := &fakeErasureFinder{byEmail: map[string]string{"jo@example.com": "auth0|jo"}}

	rec := eraseRequest(t, finder, newFakeDeleters(&calls, nil), "k", `{"email":"jo@example.com"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	if out["userId"] != "auth0|jo" {
		t.Errorf("userId = %v", out["userId"])
	}
	want := []string{"notifications", "watchzones", "saved", "devices", "state", "offercodes", "profile", "auth0"}
	if len(calls) != len(want) {
		t.Fatalf("calls = %v", calls)
	}
	for i, name := range want {
		if calls[i] != name+":auth0|jo" {
			t.Errorf("calls[%d] = %q, want %q", i, calls[i], name+":auth0|jo")
		}
	}
}

func TestErasure_TrimsTheEmailBeforeTheLookup(t *testing.T) {
	t.Parallel()
	var calls []string
	finder := &fakeErasureFinder{byEmail: map[string]string{"jo@example.com": "auth0|jo"}}

	rec := eraseRequest(t, finder, newFakeDeleters(&calls, nil), "k", `{"email":"  jo@example.com \n"}`)

	if rec.Code != http.StatusOK || len(finder.asked) != 1 || finder.asked[0] != "jo@example.com" {
		t.Fatalf("status = %d asked = %q", rec.Code, finder.asked)
	}
}

func TestErasure_UnknownEmailIs404AndErasesNothing(t *testing.T) {
	t.Parallel()
	var calls []string
	finder := &fakeErasureFinder{byEmail: map[string]string{}}

	rec := eraseRequest(t, finder, newFakeDeleters(&calls, nil), "k", `{"email":"nobody@example.com"}`)

	if rec.Code != http.StatusNotFound || len(calls) != 0 {
		t.Fatalf("status = %d calls = %v", rec.Code, calls)
	}
}

func TestErasure_RejectsBadRequestsWithoutErasing(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		key  string
		body string
		want int
	}{
		"missing key":   {key: "", body: `{"email":"jo@example.com"}`, want: http.StatusUnauthorized},
		"wrong key":     {key: "nope", body: `{"email":"jo@example.com"}`, want: http.StatusUnauthorized},
		"bad json":      {key: "k", body: `{`, want: http.StatusBadRequest},
		"missing email": {key: "k", body: `{}`, want: http.StatusBadRequest},
		"blank email":   {key: "k", body: `{"email":"   "}`, want: http.StatusBadRequest},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var calls []string
			finder := &fakeErasureFinder{byEmail: map[string]string{"jo@example.com": "auth0|jo"}}

			rec := eraseRequest(t, finder, newFakeDeleters(&calls, nil), tc.key, tc.body)

			if rec.Code != tc.want || len(calls) != 0 {
				t.Fatalf("status = %d calls = %v", rec.Code, calls)
			}
		})
	}
}

func TestErasure_LookupFailureIs500(t *testing.T) {
	t.Parallel()
	var calls []string
	finder := &fakeErasureFinder{err: errors.New("db down")}

	rec := eraseRequest(t, finder, newFakeDeleters(&calls, nil), "k", `{"email":"jo@example.com"}`)

	if rec.Code != http.StatusInternalServerError || len(calls) != 0 {
		t.Fatalf("status = %d calls = %v", rec.Code, calls)
	}
}

func TestErasure_CascadeFailureIs500(t *testing.T) {
	t.Parallel()
	var calls []string
	finder := &fakeErasureFinder{byEmail: map[string]string{"jo@example.com": "auth0|jo"}}

	rec := eraseRequest(t, finder, newFakeDeleters(&calls, errors.New("auth0 down")), "k", `{"email":"jo@example.com"}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
}
