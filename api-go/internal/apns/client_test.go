package apns

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"log/slog"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newTestClient builds a Client whose transport targets the given httptest
// server. The server speaks HTTP/1.1; the client's request-construction and
// response-handling logic is identical regardless of protocol, so HTTP/2 is not
// needed to exercise it.
func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	pemBytes, _ := newTestKeyPEM(t)
	opts := Options{
		Enabled:  true,
		AuthKey:  string(pemBytes),
		KeyID:    "L2J5PQASN5",
		TeamID:   "4574VQ7N2X",
		BundleID: "uk.towncrierapp.mobile",
	}
	client, err := newClientWithBaseURL(opts, srv.URL, srv.Client(), testLogger(), func() time.Time {
		return time.Unix(1_700_000_000, 0).UTC()
	})
	if err != nil {
		t.Fatalf("newClientWithBaseURL: %v", err)
	}
	return client
}

func TestClient_SendsCorrectRequestShape(t *testing.T) {
	t.Parallel()

	var (
		gotPath   string
		gotAuth   string
		gotTopic  string
		gotType   string
		gotBody   []byte
		mu        sync.Mutex
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("authorization")
		gotTopic = r.Header.Get("apns-topic")
		gotType = r.Header.Get("apns-push-type")
		gotBody = body
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	payload := json.RawMessage(`{"aps":{"alert":"hi"}}`)

	invalid, err := client.Send(context.Background(), []string{"abc123token"}, payload)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(invalid) != 0 {
		t.Fatalf("invalid = %v, want none", invalid)
	}

	mu.Lock()
	defer mu.Unlock()
	if gotPath != "/3/device/abc123token" {
		t.Errorf("path = %q, want /3/device/abc123token", gotPath)
	}
	if !strings.HasPrefix(gotAuth, "bearer ") {
		t.Errorf("authorization = %q, want bearer-prefixed", gotAuth)
	}
	if gotTopic != "uk.towncrierapp.mobile" {
		t.Errorf("apns-topic = %q, want uk.towncrierapp.mobile", gotTopic)
	}
	if gotType != "alert" {
		t.Errorf("apns-push-type = %q, want alert", gotType)
	}
	if string(gotBody) != string(payload) {
		t.Errorf("body = %q, want %q", gotBody, payload)
	}
}

func TestClient_ReportsUnregisteredTokenAsInvalid(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone) // 410 Unregistered
		_, _ = w.Write([]byte(`{"reason":"Unregistered"}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	invalid, err := client.Send(context.Background(), []string{"deadtoken"}, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(invalid) != 1 || invalid[0] != "deadtoken" {
		t.Fatalf("invalid = %v, want [deadtoken]", invalid)
	}
}

func TestClient_ReportsBadDeviceTokenAsInvalid(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"reason":"BadDeviceToken"}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	invalid, err := client.Send(context.Background(), []string{"badtoken"}, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(invalid) != 1 || invalid[0] != "badtoken" {
		t.Fatalf("invalid = %v, want [badtoken]", invalid)
	}
}

func TestClient_RefreshesJWTOnExpiredProviderToken(t *testing.T) {
	t.Parallel()

	var (
		mu    sync.Mutex
		auths []string
		calls int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		auths = append(auths, r.Header.Get("authorization"))
		calls++
		n := calls
		mu.Unlock()
		if n == 1 {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"reason":"ExpiredProviderToken"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	invalid, err := client.Send(context.Background(), []string{"tok"}, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(invalid) != 0 {
		t.Fatalf("invalid = %v, want none after retry", invalid)
	}

	mu.Lock()
	defer mu.Unlock()
	if calls != 2 {
		t.Fatalf("calls = %d, want 2 (one expired, one retry)", calls)
	}
}

func TestClient_RetriesOn5xx(t *testing.T) {
	t.Parallel()

	var (
		mu    sync.Mutex
		calls int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		if n < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	invalid, err := client.Send(context.Background(), []string{"tok"}, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(invalid) != 0 {
		t.Fatalf("invalid = %v, want none after retry", invalid)
	}

	mu.Lock()
	defer mu.Unlock()
	if calls != 2 {
		t.Fatalf("calls = %d, want 2 (one 5xx, one retry)", calls)
	}
}

func TestClient_EmptyTokensIsNoop(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called with no tokens")
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	invalid, err := client.Send(context.Background(), nil, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(invalid) != 0 {
		t.Fatalf("invalid = %v, want none", invalid)
	}
}

func TestNoOpSender_SendReturnsNoInvalidTokens(t *testing.T) {
	t.Parallel()

	sender := NewNoOpSender()
	invalid, err := sender.Send(context.Background(), []string{"a", "b"}, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(invalid) != 0 {
		t.Fatalf("invalid = %v, want none", invalid)
	}
}
