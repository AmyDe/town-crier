package webhooks

import (
	"io"
	"net/http"
	"testing"
	"time"
)

const testSecret = "tcwh_AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8"

func TestSign_KnownValue(t *testing.T) {
	t.Parallel()
	got := Sign(testSecret, 1795701600, []byte(`{"schemaVersion":1,"eventId":"e-1"}`))
	const want = "v1=0fa9f5bf4ab8b38b3f02f989cf0d8791e11d14dfd8e2c2cef494bb486d58bd74"
	if got != want {
		t.Errorf("Sign = %s, want %s", got, want)
	}
}

func testMessage() Message {
	return Message{
		URL:       "https://hooks.example.com/abc",
		Secret:    testSecret,
		EventID:   "e-1",
		EventType: EventApplicationNew,
		Body:      []byte(`{"schemaVersion":1,"eventId":"e-1"}`),
	}
}

func readBody(t *testing.T, r *http.Request) string {
	t.Helper()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestNewRequest_Headers(t *testing.T) {
	t.Parallel()
	msg := testMessage()
	req, err := newRequest(t.Context(), msg, time.Unix(1795701600, 999))
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != http.MethodPost || req.URL.String() != msg.URL {
		t.Errorf("request = %s %s", req.Method, req.URL)
	}
	for header, want := range map[string]string{
		"Content-Type":           "application/json",
		"User-Agent":             "TownCrier-Webhooks/1.0 (+https://towncrierapp.uk/developers/webhooks)",
		"X-TownCrier-Event-Id":   "e-1",
		"X-TownCrier-Event-Type": "application.new",
		"X-TownCrier-Timestamp":  "1795701600",
		"X-TownCrier-Signature":  "v1=0fa9f5bf4ab8b38b3f02f989cf0d8791e11d14dfd8e2c2cef494bb486d58bd74",
	} {
		if got := req.Header.Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if got := readBody(t, req); got != string(msg.Body) {
		t.Errorf("body = %s", got)
	}
}

func TestNewRequest_RetryKeepsBodyAndEventID(t *testing.T) {
	t.Parallel()
	msg := testMessage()
	first, err := newRequest(t.Context(), msg, time.Unix(1795701600, 0))
	if err != nil {
		t.Fatal(err)
	}
	retry, err := newRequest(t.Context(), msg, time.Unix(1795705200, 0))
	if err != nil {
		t.Fatal(err)
	}
	if readBody(t, first) != readBody(t, retry) {
		t.Error("retry changed the body")
	}
	if first.Header.Get(headerEventID) != retry.Header.Get(headerEventID) {
		t.Error("retry changed the event id")
	}
	if first.Header.Get(headerSignature) == retry.Header.Get(headerSignature) {
		t.Error("signature must cover the new timestamp")
	}
}
