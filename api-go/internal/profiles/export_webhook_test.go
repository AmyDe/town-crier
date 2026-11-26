package profiles

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeWebhookReader struct {
	webhook ExportedWebhook
	found   bool
	err     error
}

func (f fakeWebhookReader) WebhookByUser(context.Context, string) (ExportedWebhook, bool, error) {
	return f.webhook, f.found, f.err
}

func exportBody(t *testing.T, readers ExportReaders) (int, map[string]json.RawMessage) {
	t.Helper()
	store := newFakeStore()
	store.byID["auth0|abc"] = &UserProfile{
		UserID: "auth0|abc", Preferences: DefaultPreferences(), Tier: TierPro, LastActiveAt: time.Now(),
	}
	h := newTestHandlerExport(store, newFakeAuth0(), readers)
	rec := httptest.NewRecorder()
	h.exportData(rec, withSubject(http.MethodGet, "/v1/me/data", "auth0|abc", ""))
	body := map[string]json.RawMessage{}
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
	}
	return rec.Code, body
}

func TestHandler_ExportData_Webhook(t *testing.T) {
	t.Parallel()
	configured := fakeWebhookReader{found: true, webhook: ExportedWebhook{
		URL: "https://hooks.example.com/abc", Format: "json", Status: "active",
		CreatedAt: dnt(t, "2026-11-26T14:00:00Z"),
	}}
	tests := []struct {
		name    string
		readers ExportReaders
		want    string
	}{
		{
			"configured",
			ExportReaders{Webhook: configured},
			`{"url":"https://hooks.example.com/abc","format":"json","status":"active","createdAt":"2026-11-26T14:00:00+00:00"}`,
		},
		{"none set", ExportReaders{Webhook: fakeWebhookReader{}}, `null`},
		{"no reader wired", ExportReaders{}, `null`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			code, body := exportBody(t, tc.readers)

			got, present := body["webhook"]
			if code != http.StatusOK || !present || string(got) != tc.want {
				t.Fatalf("status = %d webhook = %s (present %v), want %s", code, got, present, tc.want)
			}
		})
	}
}

func TestHandler_ExportData_WebhookReadFailureIs500(t *testing.T) {
	t.Parallel()

	code, _ := exportBody(t, ExportReaders{Webhook: fakeWebhookReader{err: errors.New("db down")}})

	if code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", code)
	}
}
