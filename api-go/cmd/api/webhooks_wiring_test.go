package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/devicetokens"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/notificationstate"
	"github.com/AmyDe/town-crier/api-go/internal/offercodes"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/savedapplications"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
	"github.com/AmyDe/town-crier/api-go/internal/webhooks"
)

const wiringWebhookSecret = "tcwh_wiring-secret-value"

type fakeWebhookStore struct {
	endpoint *webhooks.Endpoint
	err      error
}

func (f fakeWebhookStore) ByUserID(context.Context, string) (webhooks.Endpoint, error) {
	if f.err != nil {
		return webhooks.Endpoint{}, f.err
	}
	if f.endpoint == nil {
		return webhooks.Endpoint{}, webhooks.ErrNotFound
	}
	return *f.endpoint, nil
}
func (fakeWebhookStore) Create(context.Context, webhooks.Endpoint) error { return nil }
func (fakeWebhookStore) Replace(context.Context, string, string, webhooks.Format, time.Time) (webhooks.Endpoint, error) {
	return webhooks.Endpoint{}, webhooks.ErrNotFound
}
func (fakeWebhookStore) RotateSecret(context.Context, string, string, time.Time) error { return nil }
func (fakeWebhookStore) ClaimTest(context.Context, string, time.Time) (bool, error)    { return true, nil }
func (fakeWebhookStore) DeleteAllByUserID(context.Context, string) error               { return nil }

type fakeWebhookSender struct{}

func (fakeWebhookSender) Send(context.Context, webhooks.Message) webhooks.Result {
	return webhooks.Result{}
}

type recordingEraseStep struct {
	label string
	calls *[]string
}

func (s recordingEraseStep) DeleteAllByUserID(_ context.Context, userID string) error {
	*s.calls = append(*s.calls, s.label+":"+userID)
	return nil
}

func (s recordingEraseStep) AnonymiseRedemptionsByUserID(ctx context.Context, userID string) error {
	return s.DeleteAllByUserID(ctx, userID)
}

func recordingCascadeDeleters(calls *[]string) profiles.CascadeDeleters {
	step := func(label string) recordingEraseStep { return recordingEraseStep{label: label, calls: calls} }
	return profiles.CascadeDeleters{
		Notifications:       step("notifications"),
		WatchZones:          step("watchZones"),
		SavedApplications:   step("saved"),
		DeviceRegistrations: step("devices"),
		NotificationState:   step("state"),
		Webhooks:            step("webhooks"),
		OfferCodes:          step("offerCodes"),
	}
}

func webhookRouter(t *testing.T, cascade profiles.CascadeDeleters, readers profiles.ExportReaders, hooks webhookDeps) http.Handler {
	t.Helper()
	validator := staticValidator{claims: auth.Claims{Subject: "auth0|wiretest", Email: "wire@example.com", EmailVerified: true}}
	return newRouter(validator, []string{"https://towncrierapp.uk"}, newFakeProfileStore(), profiles.NoOpAuth0Client{}, cascade, readers, nil, nil, nil, nil, nil, nil, testGeocodeClient(t), testDesignationClient(t), nil, fakeAdminStore{}, "s3cret", "", nil, nil, "", nil, nil, nil, 60, 60, pollingAdminDeps{}, nil, nil, nil, nil, hooks, slog.New(slog.DiscardHandler))
}

func serveBody(t *testing.T, h http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req := httptest.NewRequestWithContext(ctx, method, path, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRouter_WebhookRoutes(t *testing.T) {
	t.Parallel()
	h := webhookRouter(t, profiles.CascadeDeleters{}, profiles.ExportReaders{},
		webhookDeps{store: fakeWebhookStore{}, sender: fakeWebhookSender{}})
	bearer := map[string]string{"Authorization": "Bearer tok"}
	if rec := serveBody(t, h, http.MethodPost, "/v1/me", "", bearer); rec.Code >= 300 {
		t.Fatalf("create profile: %d %s", rec.Code, rec.Body.String())
	}

	tests := []struct {
		method, path, body string
		wantStatus         int
		wantBody           string
	}{
		{http.MethodGet, "/v1/me/webhook", "", http.StatusNotFound, "webhook_not_found"},
		{http.MethodPut, "/v1/me/webhook", `{"url":"https://hooks.example.com/a","format":"json"}`, http.StatusForbidden, "webhook_requires_pro"},
		{http.MethodDelete, "/v1/me/webhook", "", http.StatusNoContent, ""},
		{http.MethodPost, "/v1/me/webhook/secret", "", http.StatusForbidden, "webhook_requires_pro"},
		{http.MethodPost, "/v1/me/webhook/test", "", http.StatusForbidden, "webhook_requires_pro"},
	}
	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			t.Parallel()
			if rec := serveBody(t, h, tc.method, tc.path, tc.body, nil); rec.Code != http.StatusUnauthorized {
				t.Errorf("without a token: status = %d, want 401", rec.Code)
			}

			rec := serveBody(t, h, tc.method, tc.path, tc.body, bearer)

			if rec.Code != tc.wantStatus || !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Errorf("status = %d body = %s, want %d %q", rec.Code, rec.Body.String(), tc.wantStatus, tc.wantBody)
			}
			if rec.Header().Get("X-RateLimit-Limit") == "" {
				t.Error("X-RateLimit-Limit missing: the per-subject limiter is not in the path")
			}
		})
	}
}

func TestRouter_BothAPIErasureRoutesDeleteTheWebhook(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, method, path, body string
		headers                  map[string]string
		userID                   string
	}{
		{"DELETE /v1/me", http.MethodDelete, "/v1/me", "", map[string]string{"Authorization": "Bearer tok"}, "auth0|wiretest"},
		{"admin erase", http.MethodPost, "/v1/admin/users/erase", `{"userId":"auth0|gone"}`, map[string]string{"X-Admin-Key": "s3cret"}, "auth0|gone"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls []string
			h := webhookRouter(t, recordingCascadeDeleters(&calls), profiles.ExportReaders{}, webhookDeps{})

			rec := serveBody(t, h, tc.method, tc.path, tc.body, tc.headers)

			if rec.Code >= 300 || !slices.Contains(calls, "webhooks:"+tc.userID) {
				t.Fatalf("status = %d calls = %v body = %s, want the webhook erased", rec.Code, calls, rec.Body.String())
			}
		})
	}
}

func TestRouter_ExportIncludesWebhookWithoutItsSecret(t *testing.T) {
	t.Parallel()
	endpoint := &webhooks.Endpoint{
		ID: "e-1", UserID: "auth0|wiretest", URL: "https://hooks.example.com/abc", Format: webhooks.FormatSlack,
		Secret: wiringWebhookSecret, Status: webhooks.StatusDisabled,
		CreatedAt: time.Date(2026, 11, 26, 14, 0, 0, 0, time.UTC),
	}
	readers := profiles.ExportReaders{Webhook: webhookExportReader{store: fakeWebhookStore{endpoint: endpoint}}}
	h := webhookRouter(t, profiles.CascadeDeleters{}, readers, webhookDeps{})
	bearer := map[string]string{"Authorization": "Bearer tok"}
	serveBody(t, h, http.MethodPost, "/v1/me", "", bearer)

	rec := serveBody(t, h, http.MethodGet, "/v1/me/data", "", bearer)

	var body struct {
		Webhook json.RawMessage `json:"webhook"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("status = %d: %v", rec.Code, err)
	}
	want := `{"url":"https://hooks.example.com/abc","format":"slack","status":"disabled","createdAt":"2026-11-26T14:00:00+00:00"}`
	if string(body.Webhook) != want {
		t.Errorf("webhook = %s, want %s", body.Webhook, want)
	}
	if strings.Contains(rec.Body.String(), wiringWebhookSecret) || strings.Contains(rec.Body.String(), "tcwh_") {
		t.Error("the export contains the signing secret")
	}
}

func TestWebhookExportReader_NoEndpointAndStoreFailure(t *testing.T) {
	t.Parallel()

	_, found, err := webhookExportReader{store: fakeWebhookStore{}}.WebhookByUser(context.Background(), "u")
	if found || err != nil {
		t.Errorf("no endpoint: found = %v err = %v", found, err)
	}

	down := errors.New("db down")
	_, _, err = webhookExportReader{store: fakeWebhookStore{err: down}}.WebhookByUser(context.Background(), "u")
	if !errors.Is(err, down) {
		t.Errorf("err = %v, want the store error", err)
	}
}

func TestGDPRWiring_EveryErasureStepAndExportReaderIsWired(t *testing.T) {
	t.Parallel()
	hooks := webhooks.NewPostgresStore(nil)

	cascade, readers := gdprWiring(
		notifications.NewPostgresStore(nil),
		watchzones.NewPostgresStore(nil),
		savedapplications.NewPostgresStore(nil),
		devicetokens.NewPostgresStore(nil),
		notificationstate.NewPostgresStore(nil),
		offercodes.NewPostgresStore(nil, slog.New(slog.DiscardHandler)),
		hooks,
	)

	for _, bundle := range []any{cascade, readers} {
		fields := reflect.ValueOf(bundle)
		for i := range fields.NumField() {
			if fields.Field(i).IsZero() {
				t.Errorf("%s.%s is not wired", fields.Type().Name(), fields.Type().Field(i).Name)
			}
		}
	}
	if cascade.Webhooks != hooks {
		t.Errorf("webhook deleter = %T, want the webhook store", cascade.Webhooks)
	}
	if reader, ok := readers.Webhook.(webhookExportReader); !ok || reader.store != hooks {
		t.Errorf("webhook export reader = %T, want it to read the webhook store", readers.Webhook)
	}
}
