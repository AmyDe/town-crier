package admin

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/erasure"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

type fakeErasureFinder struct {
	byEmail map[string][]string
	err     error
	asked   []string
}

func (f *fakeErasureFinder) GetByEmail(_ context.Context, email string) (*profiles.UserProfile, error) {
	f.asked = append(f.asked, email)
	if f.err != nil {
		return nil, f.err
	}
	ids := f.byEmail[email]
	if len(ids) == 0 {
		return nil, profiles.ErrNotFound
	}
	return &profiles.UserProfile{UserID: ids[0], Email: &email}, nil
}

func (f *fakeErasureFinder) remove(userID string) {
	for email, ids := range f.byEmail {
		f.byEmail[email] = slices.DeleteFunc(ids, func(id string) bool { return id == userID })
	}
}

type fakeEraseStep struct {
	name    string
	calls   *[]string
	err     error
	onCall  func(userID string)
	failFor string
}

func (s fakeEraseStep) record(userID string) error {
	*s.calls = append(*s.calls, s.name+":"+userID)
	if s.failFor != "" && s.failFor == userID {
		return s.err
	}
	if s.failFor == "" && s.err != nil {
		return s.err
	}
	if s.onCall != nil {
		s.onCall(userID)
	}
	return nil
}

func (s fakeEraseStep) DeleteAllByUserID(_ context.Context, userID string) error {
	return s.record(userID)
}

func (s fakeEraseStep) AnonymiseRedemptionsByUserID(_ context.Context, userID string) error {
	return s.record(userID)
}

func (s fakeEraseStep) Delete(_ context.Context, userID string) error { return s.record(userID) }

func (s fakeEraseStep) DeleteUser(_ context.Context, userID string) error { return s.record(userID) }

type erasureFixture struct {
	finder      *fakeErasureFinder
	calls       []string
	auth0Err    error
	auth0FailID string
	keepProfile bool
}

func (f *erasureFixture) deleters() erasure.Deleters {
	step := func(name string) fakeEraseStep { return fakeEraseStep{name: name, calls: &f.calls} }
	profile := step("profile")
	if !f.keepProfile {
		profile.onCall = f.finder.remove
	}
	return erasure.Deleters{
		Notifications:       step("notifications"),
		WatchZones:          step("watchzones"),
		SavedApplications:   step("saved"),
		DeviceRegistrations: step("devices"),
		NotificationState:   step("state"),
		OfferCodes:          step("offercodes"),
		Profile:             profile,
		Auth0:               fakeEraseStep{name: "auth0", calls: &f.calls, err: f.auth0Err, failFor: f.auth0FailID},
	}
}

func (f *erasureFixture) post(t *testing.T, key, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	mux := http.NewServeMux()
	ErasureRoutes(mux, "k", f.finder, f.deleters(), slog.New(slog.DiscardHandler))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/admin/users/erase", strings.NewReader(body))
	if key != "" {
		req.Header.Set(adminKeyHeader, key)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var out map[string]any
	if strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode %q: %v", rec.Body.String(), err)
		}
	}
	return rec, out
}

func newErasureFixture(byEmail map[string][]string) *erasureFixture {
	return &erasureFixture{finder: &fakeErasureFinder{byEmail: byEmail}}
}

func stepsFor(userID string) []string {
	names := []string{"notifications", "watchzones", "saved", "devices", "state", "offercodes", "profile", "auth0"}
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, name+":"+userID)
	}
	return out
}

func ids(v any) []string {
	list, _ := v.([]any)
	out := make([]string, 0, len(list))
	for _, x := range list {
		s, _ := x.(string)
		out = append(out, s)
	}
	return out
}

func TestErasure_ErasesEveryStepForTheUserFoundByEmail(t *testing.T) {
	t.Parallel()
	f := newErasureFixture(map[string][]string{"jo@example.com": {"auth0|jo"}})

	rec, out := f.post(t, "k", `{"email":"jo@example.com"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := ids(out["userIds"]); !slices.Equal(got, []string{"auth0|jo"}) {
		t.Errorf("userIds = %v", got)
	}
	if !slices.Equal(f.calls, stepsFor("auth0|jo")) {
		t.Errorf("calls = %v", f.calls)
	}
}

func TestErasure_ErasesEveryProfileThatSharesTheEmail(t *testing.T) {
	t.Parallel()
	f := newErasureFixture(map[string][]string{"jo@example.com": {"auth0|new", "auth0|old"}})

	rec, out := f.post(t, "k", `{"email":"jo@example.com"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := ids(out["userIds"]); !slices.Equal(got, []string{"auth0|new", "auth0|old"}) {
		t.Errorf("userIds = %v", got)
	}
	if want := append(stepsFor("auth0|new"), stepsFor("auth0|old")...); !slices.Equal(f.calls, want) {
		t.Errorf("calls = %v", f.calls)
	}
}

func TestErasure_TrimsTheEmailBeforeTheLookup(t *testing.T) {
	t.Parallel()
	f := newErasureFixture(map[string][]string{"jo@example.com": {"auth0|jo"}})

	rec, _ := f.post(t, "k", `{"email":"  jo@example.com \n"}`)

	if rec.Code != http.StatusOK || f.finder.asked[0] != "jo@example.com" {
		t.Fatalf("status = %d asked = %q", rec.Code, f.finder.asked)
	}
}

func TestErasure_UnknownEmailIs404AndErasesNothing(t *testing.T) {
	t.Parallel()
	f := newErasureFixture(map[string][]string{})

	rec, _ := f.post(t, "k", `{"email":"nobody@example.com"}`)

	if rec.Code != http.StatusNotFound || len(f.calls) != 0 {
		t.Fatalf("status = %d calls = %v", rec.Code, f.calls)
	}
}

func TestErasure_UserIDRunsTheCascadeWithoutAProfile(t *testing.T) {
	t.Parallel()
	f := newErasureFixture(map[string][]string{})

	rec, out := f.post(t, "k", `{"userId":" auth0|gone "}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := ids(out["userIds"]); !slices.Equal(got, []string{"auth0|gone"}) {
		t.Errorf("userIds = %v", got)
	}
	if !slices.Equal(f.calls, stepsFor("auth0|gone")) || len(f.finder.asked) != 0 {
		t.Errorf("calls = %v asked = %v", f.calls, f.finder.asked)
	}
}

func TestErasure_RejectsBadRequestsWithoutErasing(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		key  string
		body string
		want int
	}{
		"missing key":          {key: "", body: `{"email":"jo@example.com"}`, want: http.StatusUnauthorized},
		"wrong key":            {key: "nope", body: `{"email":"jo@example.com"}`, want: http.StatusUnauthorized},
		"bad json":             {key: "k", body: `{`, want: http.StatusBadRequest},
		"no target":            {key: "k", body: `{}`, want: http.StatusBadRequest},
		"blank email":          {key: "k", body: `{"email":"   "}`, want: http.StatusBadRequest},
		"email and user id":    {key: "k", body: `{"email":"jo@example.com","userId":"auth0|jo"}`, want: http.StatusBadRequest},
		"blank user id only":   {key: "k", body: `{"userId":" "}`, want: http.StatusBadRequest},
		"blank email, user id": {key: "k", body: `{"email":"","userId":""}`, want: http.StatusBadRequest},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newErasureFixture(map[string][]string{"jo@example.com": {"auth0|jo"}})

			rec, _ := f.post(t, tc.key, tc.body)

			if rec.Code != tc.want || len(f.calls) != 0 {
				t.Fatalf("status = %d calls = %v", rec.Code, f.calls)
			}
		})
	}
}

func TestErasure_LookupFailureIs500(t *testing.T) {
	t.Parallel()
	f := newErasureFixture(nil)
	f.finder.err = errors.New("db down")

	rec, out := f.post(t, "k", `{"email":"jo@example.com"}`)

	if rec.Code != http.StatusInternalServerError || len(f.calls) != 0 {
		t.Fatalf("status = %d calls = %v", rec.Code, f.calls)
	}
	if _, ok := out["failedUserId"]; ok {
		t.Errorf("failedUserId present on a lookup failure: %v", out)
	}
}

func TestErasure_CascadeFailureReportsWhatWasAndWasNotErased(t *testing.T) {
	t.Parallel()
	f := newErasureFixture(map[string][]string{"jo@example.com": {"auth0|new", "auth0|old"}})
	f.auth0Err, f.auth0FailID = errors.New("auth0 down"), "auth0|old"

	rec, out := f.post(t, "k", `{"email":"jo@example.com"}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := ids(out["erasedUserIds"]); !slices.Equal(got, []string{"auth0|new"}) {
		t.Errorf("erasedUserIds = %v", got)
	}
	if out["failedUserId"] != "auth0|old" {
		t.Errorf("failedUserId = %v", out["failedUserId"])
	}
}

func TestErasure_UserIDCascadeFailureIs500WithTheFailedID(t *testing.T) {
	t.Parallel()
	f := newErasureFixture(nil)
	f.auth0Err = errors.New("auth0 down")

	rec, out := f.post(t, "k", `{"userId":"auth0|jo"}`)

	if rec.Code != http.StatusInternalServerError || out["failedUserId"] != "auth0|jo" {
		t.Fatalf("status = %d out = %v", rec.Code, out)
	}
}

func TestErasure_ProfileThatSurvivesTheCascadeStopsTheLoop(t *testing.T) {
	t.Parallel()
	f := newErasureFixture(map[string][]string{"jo@example.com": {"auth0|jo"}})
	f.keepProfile = true

	rec, out := f.post(t, "k", `{"email":"jo@example.com"}`)

	if rec.Code != http.StatusInternalServerError || out["failedUserId"] != "auth0|jo" {
		t.Fatalf("status = %d out = %v", rec.Code, out)
	}
	if !slices.Equal(f.calls, stepsFor("auth0|jo")) {
		t.Errorf("calls = %v, want one cascade", f.calls)
	}
}
