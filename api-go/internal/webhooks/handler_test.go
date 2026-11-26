package webhooks

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
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

const testUser = "auth0|owner"

// fakeEndpointStore holds at most one endpoint, for testUser.
type fakeEndpointStore struct {
	endpoint *Endpoint

	readErr, createErr, replaceErr, rotateErr, claimErr, deleteErr error

	// createdFirst simulates a concurrent create that wins the race.
	createdFirst *Endpoint
	claimedAt    []time.Time
}

func (f *fakeEndpointStore) own(userID string) *Endpoint {
	if f.endpoint == nil || f.endpoint.UserID != userID {
		return nil
	}
	return f.endpoint
}

func (f *fakeEndpointStore) ByUserID(_ context.Context, userID string) (Endpoint, error) {
	if f.readErr != nil {
		return Endpoint{}, f.readErr
	}
	if e := f.own(userID); e != nil {
		return *e, nil
	}
	return Endpoint{}, ErrNotFound
}

func (f *fakeEndpointStore) Create(_ context.Context, e Endpoint) error {
	if f.createErr != nil {
		return f.createErr
	}
	if f.createdFirst != nil {
		f.endpoint = f.createdFirst
		return ErrEndpointExists
	}
	f.endpoint = &e
	return nil
}

func (f *fakeEndpointStore) Replace(_ context.Context, userID, url string, format Format, now time.Time) (Endpoint, error) {
	if f.replaceErr != nil {
		return Endpoint{}, f.replaceErr
	}
	e := f.own(userID)
	if e == nil {
		return Endpoint{}, ErrNotFound
	}
	e.URL, e.Format, e.UpdatedAt = url, format, now
	e.Status, e.DisabledReason, e.ConsecutiveFailures = StatusActive, nil, 0
	return *e, nil
}

func (f *fakeEndpointStore) RotateSecret(_ context.Context, userID, secret string, _ time.Time) error {
	if f.rotateErr != nil {
		return f.rotateErr
	}
	e := f.own(userID)
	if e == nil {
		return ErrNotFound
	}
	e.Secret = secret
	return nil
}

func (f *fakeEndpointStore) ClaimTest(_ context.Context, userID string, now time.Time) (bool, error) {
	if f.claimErr != nil {
		return false, f.claimErr
	}
	e := f.own(userID)
	if e == nil || (e.LastTestAt != nil && now.Sub(*e.LastTestAt) < TestCooldown) {
		return false, nil
	}
	e.LastTestAt = &now
	f.claimedAt = append(f.claimedAt, now)
	return true, nil
}

func (f *fakeEndpointStore) DeleteAllByUserID(_ context.Context, userID string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if f.own(userID) != nil {
		f.endpoint = nil
	}
	return nil
}

var handlerNow = time.Date(2026, 11, 26, 14, 0, 0, 0, time.UTC)

func proProfile() fakeProfiles {
	return fakeProfiles{profile: &profiles.UserProfile{Tier: profiles.TierPro}}
}

func lapsedProProfile() fakeProfiles {
	return fakeProfiles{profile: &profiles.UserProfile{
		Tier: profiles.TierPro, SubscriptionExpiry: new(handlerNow.Add(-time.Hour)),
	}}
}

type handlerFixture struct {
	store    *fakeEndpointStore
	profiles fakeProfiles
	sender   *fakeMessageSender
	now      time.Time
	logs     *syncBuffer
}

func newHandlerFixture() *handlerFixture {
	return &handlerFixture{
		store:    &fakeEndpointStore{},
		profiles: proProfile(),
		sender:   &fakeMessageSender{},
		now:      handlerNow,
		logs:     &syncBuffer{},
	}
}

func (fx *handlerFixture) withEndpoint() *Endpoint {
	fx.store.endpoint = &Endpoint{
		ID: "endpoint-1", UserID: testUser, URL: "https://hooks.example.com/abc", Format: FormatJSON,
		Secret: testSecret, Status: StatusActive, CreatedAt: handlerNow, UpdatedAt: handlerNow,
	}
	return fx.store.endpoint
}

func (fx *handlerFixture) serve(ctx context.Context, method, path, body string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	logger := slog.New(slog.NewJSONHandler(fx.logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	Routes(mux, fx.store, fx.profiles, fx.sender, func() string { return "new-id" },
		func() time.Time { return fx.now }, logger)
	req := httptest.NewRequestWithContext(auth.WithSubject(ctx, testUser), method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func (fx *handlerFixture) do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	return fx.serve(t.Context(), method, path, body)
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("body %q: %v", rec.Body.String(), err)
	}
	return out
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, want, rec.Body.String())
	}
}

func assertErrorCode(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	assertStatus(t, rec, status)
	body := decodeBody(t, rec)
	if body["error"] != code {
		t.Errorf("error = %v, want %s", body["error"], code)
	}
	if message, _ := body["message"].(string); message == "" {
		t.Error("no message")
	}
}

var responseKeys = []string{
	"url", "format", "status", "disabledReason", "paused", "createdAt",
	"lastSuccessAt", "lastFailureAt", "lastFailureReason", "lastFailureStatus",
}

const validPut = `{"url":"https://hooks.example.com/abc","format":"json"}`

func TestHandler_Get(t *testing.T) {
	t.Parallel()
	t.Run("no webhook", func(t *testing.T) {
		t.Parallel()
		assertErrorCode(t, newHandlerFixture().do(t, "GET", "/v1/me/webhook", ""), 404, "webhook_not_found")
	})

	t.Run("new webhook", func(t *testing.T) {
		t.Parallel()
		fx := newHandlerFixture()
		fx.withEndpoint()

		rec := fx.do(t, "GET", "/v1/me/webhook", "")

		assertStatus(t, rec, 200)
		want := `{"url":"https://hooks.example.com/abc","format":"json","status":"active","disabledReason":null,` +
			`"paused":false,"createdAt":"2026-11-26T14:00:00Z","lastSuccessAt":null,"lastFailureAt":null,` +
			`"lastFailureReason":null,"lastFailureStatus":null}`
		if rec.Body.String() != want {
			t.Errorf("body = %s", rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
			t.Errorf("content type = %q", ct)
		}
	})

	t.Run("switched off webhook", func(t *testing.T) {
		t.Parallel()
		fx := newHandlerFixture()
		e := fx.withEndpoint()
		e.Status, e.DisabledReason = StatusDisabled, new(DisabledFailures)
		e.LastSuccessAt = new(handlerNow.Add(-48 * time.Hour))
		e.LastFailureAt = new(handlerNow.Add(-time.Hour))
		e.LastFailureReason, e.LastFailureStatus = new(FailureHTTPStatus), new(503)

		body := decodeBody(t, fx.do(t, "GET", "/v1/me/webhook", ""))

		for key, want := range map[string]any{
			"status": "disabled", "disabledReason": "failures", "lastSuccessAt": "2026-11-24T14:00:00Z",
			"lastFailureAt": "2026-11-26T13:00:00Z", "lastFailureReason": "http_status", "lastFailureStatus": 503.0,
		} {
			if body[key] != want {
				t.Errorf("%s = %v, want %v", key, body[key], want)
			}
		}
	})

	for name, tc := range map[string]struct {
		profiles fakeProfiles
		paused   bool
	}{
		"pro":        {proProfile(), false},
		"lapsed pro": {lapsedProProfile(), true},
		"personal":   {fakeProfiles{profile: &profiles.UserProfile{Tier: profiles.TierPersonal}}, true},
		"free":       {fakeProfiles{profile: &profiles.UserProfile{Tier: profiles.TierFree}}, true},
		"no profile": {fakeProfiles{err: profiles.ErrNotFound}, true},
	} {
		t.Run("paused for "+name, func(t *testing.T) {
			t.Parallel()
			fx := newHandlerFixture()
			fx.profiles = tc.profiles
			fx.withEndpoint()

			rec := fx.do(t, "GET", "/v1/me/webhook", "")

			assertStatus(t, rec, 200)
			if got := decodeBody(t, rec)["paused"]; got != tc.paused {
				t.Errorf("paused = %v, want %v", got, tc.paused)
			}
		})
	}
}

func TestHandler_ProRoutesRefuseOtherTiers(t *testing.T) {
	t.Parallel()
	routes := []struct{ method, path, body string }{
		{"PUT", "/v1/me/webhook", validPut},
		{"PUT", "/v1/me/webhook", `{"url":"http://bad","format":"xml"}`},
		{"POST", "/v1/me/webhook/secret", ""},
		{"POST", "/v1/me/webhook/test", ""},
	}
	for name, reader := range map[string]fakeProfiles{
		"free":       {profile: &profiles.UserProfile{Tier: profiles.TierFree}},
		"personal":   {profile: &profiles.UserProfile{Tier: profiles.TierPersonal}},
		"lapsed pro": lapsedProProfile(),
		"no profile": {err: profiles.ErrNotFound},
	} {
		for _, route := range routes {
			t.Run(name+" "+route.method+" "+route.path, func(t *testing.T) {
				t.Parallel()
				fx := newHandlerFixture()
				fx.profiles = reader
				before := *fx.withEndpoint()

				assertErrorCode(t, fx.do(t, route.method, route.path, route.body), 403, "webhook_requires_pro")

				if *fx.store.endpoint != before || len(fx.sender.sent) != 0 {
					t.Error("a refused request had an effect")
				}
			})
		}
	}
}

func TestHandler_PutCreatesWithSecret(t *testing.T) {
	t.Parallel()
	fx := newHandlerFixture()

	rec := fx.do(t, "PUT", "/v1/me/webhook", `{"url":"https://hooks.example.com/abc","format":"slack"}`)

	assertStatus(t, rec, 201)
	body := decodeBody(t, rec)
	assertKeys(t, body, append(slices.Clone(responseKeys), "secret"))
	stored := fx.store.endpoint
	if stored == nil || stored.ID != "new-id" || stored.UserID != testUser || stored.Format != FormatSlack {
		t.Fatalf("stored = %v", stored)
	}
	if secret, _ := body["secret"].(string); secret != stored.Secret || !strings.HasPrefix(secret, "tcwh_") {
		t.Errorf("secret does not match the stored one")
	}
	if body["status"] != "active" || body["paused"] != false || body["format"] != "slack" {
		t.Errorf("body = %v", body)
	}
}

func TestHandler_PutReplacesWithoutSecretAndReenables(t *testing.T) {
	t.Parallel()
	fx := newHandlerFixture()
	e := fx.withEndpoint()
	e.Status, e.DisabledReason, e.ConsecutiveFailures = StatusDisabled, new(DisabledGone), 24

	rec := fx.do(t, "PUT", "/v1/me/webhook", `{"url":"https://new.example.com/hook","format":"json"}`)

	assertStatus(t, rec, 200)
	body := decodeBody(t, rec)
	assertKeys(t, body, responseKeys)
	if body["status"] != "active" || body["disabledReason"] != nil || body["url"] != "https://new.example.com/hook" {
		t.Errorf("body = %v", body)
	}
	if got := fx.store.endpoint; got.Status != StatusActive || got.ConsecutiveFailures != 0 || got.Secret != testSecret {
		t.Errorf("stored = %s %d", got.Status, got.ConsecutiveFailures)
	}
}

func TestHandler_PutLosingACreateRaceReplaces(t *testing.T) {
	t.Parallel()
	fx := newHandlerFixture()
	winner := *newHandlerFixture().withEndpoint()
	fx.store.createdFirst = &winner

	rec := fx.do(t, "PUT", "/v1/me/webhook", `{"url":"https://new.example.com/hook","format":"json"}`)

	assertStatus(t, rec, 200)
	assertKeys(t, decodeBody(t, rec), responseKeys)
	if fx.store.endpoint.URL != "https://new.example.com/hook" {
		t.Error("the later request did not win")
	}
}

func TestHandler_PutRejectsBadInput(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body, code string
	}{
		{"http", `{"url":"http://hooks.example.com","format":"json"}`, "webhook_url_invalid"},
		{"no url", `{"format":"json"}`, "webhook_url_invalid"},
		{"private address", `{"url":"https://10.0.0.1/x","format":"json"}`, "webhook_url_invalid"},
		{"userinfo", `{"url":"https://a:b@hooks.example.com","format":"json"}`, "webhook_url_invalid"},
		{"port", `{"url":"https://hooks.example.com:8443","format":"json"}`, "webhook_url_invalid"},
		{"local name", `{"url":"https://printer.local","format":"json"}`, "webhook_url_invalid"},
		{"too long", `{"url":"https://hooks.example.com/` + strings.Repeat("a", 2048) + `","format":"json"}`, "webhook_url_invalid"},
		{"unknown format", `{"url":"https://hooks.example.com","format":"teams"}`, "webhook_format_invalid"},
		{"no format", `{"url":"https://hooks.example.com"}`, "webhook_format_invalid"},
	} {
		for _, existing := range []bool{false, true} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				fx := newHandlerFixture()
				if existing {
					fx.withEndpoint()
				}
				assertErrorCode(t, fx.do(t, "PUT", "/v1/me/webhook", tc.body), 400, tc.code)
				if e := fx.store.endpoint; (e != nil) != existing || (e != nil && e.URL != "https://hooks.example.com/abc") {
					t.Error("rejected input was saved")
				}
			})
		}
	}
}

func TestHandler_PutRejectsUnreadableBodies(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"empty":     "",
		"not json":  "{",
		"too large": `{"url":"https://hooks.example.com","format":"json","pad":"` + strings.Repeat("a", 20<<10) + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fx := newHandlerFixture()
			assertStatus(t, fx.do(t, "PUT", "/v1/me/webhook", body), 400)
			if fx.store.endpoint != nil {
				t.Error("saved")
			}
		})
	}
}

func TestHandler_Delete(t *testing.T) {
	t.Parallel()
	for name, reader := range map[string]fakeProfiles{"pro": proProfile(), "lapsed": lapsedProProfile()} {
		for _, existing := range []bool{true, false} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				fx := newHandlerFixture()
				fx.profiles = reader
				if existing {
					fx.withEndpoint()
				}
				rec := fx.do(t, "DELETE", "/v1/me/webhook", "")
				assertStatus(t, rec, 204)
				if rec.Body.Len() != 0 || fx.store.endpoint != nil {
					t.Error("not deleted")
				}
			})
		}
	}
}

func TestHandler_RotateSecret(t *testing.T) {
	t.Parallel()
	fx := newHandlerFixture()
	fx.withEndpoint()

	rec := fx.do(t, "POST", "/v1/me/webhook/secret", "")

	assertStatus(t, rec, 200)
	body := decodeBody(t, rec)
	assertKeys(t, body, []string{"secret"})
	if secret, _ := body["secret"].(string); secret == testSecret || secret != fx.store.endpoint.Secret ||
		!strings.HasPrefix(secret, "tcwh_") {
		t.Error("the returned secret is not the new stored one")
	}

	assertErrorCode(t, newHandlerFixture().do(t, "POST", "/v1/me/webhook/secret", ""), 404, "webhook_not_found")
}

func TestHandler_Test_SendsTheTestEvent(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		result Result
		want   string
	}{
		{"delivered", Result{Status: 204}, `{"delivered":true,"status":204,"failureReason":null}`},
		{"rejected", Result{Failure: FailureHTTPStatus, Status: 500}, `{"delivered":false,"status":500,"failureReason":"http_status"}`},
		{"gone", gone, `{"delivered":false,"status":410,"failureReason":"gone"}`},
		{"no response", Result{Failure: FailureTimeout}, `{"delivered":false,"status":null,"failureReason":"timeout"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fx := newHandlerFixture()
			e := fx.withEndpoint()
			e.Status, e.DisabledReason, e.ConsecutiveFailures = StatusDisabled, new(DisabledFailures), 24
			before := *e
			fx.sender = always(tc.result)

			rec := fx.do(t, "POST", "/v1/me/webhook/test", "")

			assertStatus(t, rec, 200)
			if rec.Body.String() != tc.want {
				t.Errorf("body = %s, want %s", rec.Body.String(), tc.want)
			}
			if len(fx.sender.sent) != 1 {
				t.Fatalf("sends = %d", len(fx.sender.sent))
			}
			msg := fx.sender.sent[0]
			wantBody, err := json.Marshal(BuildTestPayload("new-id", handlerNow))
			if err != nil {
				t.Fatal(err)
			}
			if msg.URL != e.URL || msg.Secret != testSecret || msg.EventID != "new-id" || msg.EventType != EventTest ||
				string(msg.Body) != string(wantBody) {
				t.Errorf("message = %s %s %s", msg.EventID, msg.EventType, msg.Body)
			}
			before.LastTestAt = new(handlerNow)
			after := *fx.store.endpoint
			if after.LastTestAt == nil || !after.LastTestAt.Equal(handlerNow) {
				t.Fatalf("last test = %v", after.LastTestAt)
			}
			after.LastTestAt = before.LastTestAt
			if after != before {
				t.Error("the test changed more than last_test_at")
			}
		})
	}
}

func TestHandler_Test_SlackBody(t *testing.T) {
	t.Parallel()
	fx := newHandlerFixture()
	fx.withEndpoint().Format = FormatSlack

	assertStatus(t, fx.do(t, "POST", "/v1/me/webhook/test", ""), 200)

	if got := slackSentText(t, fx.sender.sent[0]); got != "*Town Crier: test message*\nYour webhook works." {
		t.Errorf("text = %q", got)
	}
}

func TestHandler_Test_Cooldown(t *testing.T) {
	t.Parallel()
	fx := newHandlerFixture()
	fx.withEndpoint()

	assertStatus(t, fx.do(t, "POST", "/v1/me/webhook/test", ""), 200)
	fx.now = handlerNow.Add(9 * time.Second)
	assertErrorCode(t, fx.do(t, "POST", "/v1/me/webhook/test", ""), 429, "webhook_test_cooldown")
	fx.now = handlerNow.Add(10 * time.Second)
	assertStatus(t, fx.do(t, "POST", "/v1/me/webhook/test", ""), 200)

	if len(fx.sender.sent) != 2 {
		t.Errorf("sends = %d, want 2", len(fx.sender.sent))
	}
}

func TestHandler_Test_NoWebhook(t *testing.T) {
	t.Parallel()
	assertErrorCode(t, newHandlerFixture().do(t, "POST", "/v1/me/webhook/test", ""), 404, "webhook_not_found")
}

func TestHandler_Test_CancelledRequestIsNot200(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	fx := newHandlerFixture()
	fx.withEndpoint()
	fx.sender.respond = func(context.Context, Message) Result {
		cancel()
		return Result{Failure: FailureConnection}
	}

	assertStatus(t, fx.serve(ctx, "POST", "/v1/me/webhook/test", ""), 499)
}

func TestHandler_StoreAndProfileErrorsAre500(t *testing.T) {
	t.Parallel()
	down := errors.New("database down")
	for _, tc := range []struct {
		name         string
		method, path string
		endpoint     bool
		sabotage     func(*handlerFixture)
	}{
		{"get read", "GET", "/v1/me/webhook", true, func(f *handlerFixture) { f.store.readErr = down }},
		{"get profile", "GET", "/v1/me/webhook", true, func(f *handlerFixture) { f.profiles = fakeProfiles{err: down} }},
		{"put profile", "PUT", "/v1/me/webhook", false, func(f *handlerFixture) { f.profiles = fakeProfiles{err: down} }},
		{"put replace", "PUT", "/v1/me/webhook", true, func(f *handlerFixture) { f.store.replaceErr = down }},
		{"put create", "PUT", "/v1/me/webhook", false, func(f *handlerFixture) { f.store.createErr = down }},
		{"delete", "DELETE", "/v1/me/webhook", true, func(f *handlerFixture) { f.store.deleteErr = down }},
		{"rotate", "POST", "/v1/me/webhook/secret", true, func(f *handlerFixture) { f.store.rotateErr = down }},
		{"test read", "POST", "/v1/me/webhook/test", true, func(f *handlerFixture) { f.store.readErr = down }},
		{"test claim", "POST", "/v1/me/webhook/test", true, func(f *handlerFixture) { f.store.claimErr = down }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fx := newHandlerFixture()
			if tc.endpoint {
				fx.withEndpoint()
			}
			tc.sabotage(fx)

			assertStatus(t, fx.do(t, tc.method, tc.path, validPut), 500)

			if len(fx.sender.sent) != 0 || !strings.Contains(fx.logs.String(), "database down") {
				t.Errorf("sends = %d, logs = %s", len(fx.sender.sent), fx.logs.String())
			}
		})
	}
}
