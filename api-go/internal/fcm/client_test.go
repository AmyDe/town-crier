package fcm

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// tokenServer returns an httptest server that answers the OAuth token exchange
// with a fixed access token, counting how often it is called.
func tokenServer(t *testing.T) (*httptest.Server, func() int) {
	t.Helper()
	var (
		mu    sync.Mutex
		calls int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		_, _ = w.Write([]byte(`{"access_token":"ya29.test","expires_in":3599,"token_type":"Bearer"}`))
	}))
	t.Cleanup(srv.Close)
	return srv, func() int { mu.Lock(); defer mu.Unlock(); return calls }
}

// newTestClient builds a Client whose FCM send requests target fcmSrv and whose
// token exchange targets tokenSrv (via the service-account JSON's token_uri).
func newTestClient(t *testing.T, fcmSrv, tokenSrv *httptest.Server) *Client {
	t.Helper()
	pemBytes, _ := newTestKeyPEM(t)
	opts := Options{
		Enabled:            true,
		ProjectID:          "town-crier-test",
		ServiceAccountJSON: newTestServiceAccountJSON(t, tokenSrv.URL, pemBytes),
	}
	client, err := newClientWithBaseURL(opts, fcmSrv.URL, &http.Client{}, testLogger(), fixedClock(1_700_000_000))
	if err != nil {
		t.Fatalf("newClientWithBaseURL: %v", err)
	}
	return client
}

func TestClient_SendsCorrectRequestShape(t *testing.T) {
	t.Parallel()

	var (
		gotPath string
		gotAuth string
		gotBody []byte
		mu      sync.Mutex
	)
	fcmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("authorization")
		gotBody = body
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer fcmSrv.Close()
	tokenSrv, _ := tokenServer(t)

	client := newTestClient(t, fcmSrv, tokenSrv)
	// The message the dispatcher hands us carries no token — the client injects it.
	payload := json.RawMessage(`{"notification":{"title":"hi","body":"there"},"data":{"kind":"alert"}}`)

	invalid, err := client.Send(context.Background(), []string{"abc123token"}, payload)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(invalid) != 0 {
		t.Fatalf("invalid = %v, want none", invalid)
	}

	mu.Lock()
	defer mu.Unlock()
	if gotPath != "/v1/projects/town-crier-test/messages:send" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAuth != "Bearer ya29.test" {
		t.Errorf("authorization = %q, want Bearer ya29.test", gotAuth)
	}

	// The request body must be {"message":{...,"token":"abc123token"}} with the
	// dispatcher's message fields preserved verbatim.
	var envelope struct {
		Message struct {
			Token        string `json:"token"`
			Notification struct {
				Title string `json:"title"`
				Body  string `json:"body"`
			} `json:"notification"`
			Data struct {
				Kind string `json:"kind"`
			} `json:"data"`
		} `json:"message"`
	}
	if err := json.Unmarshal(gotBody, &envelope); err != nil {
		t.Fatalf("unmarshal body %q: %v", gotBody, err)
	}
	if envelope.Message.Token != "abc123token" {
		t.Errorf("message.token = %q, want abc123token", envelope.Message.Token)
	}
	if envelope.Message.Notification.Title != "hi" || envelope.Message.Notification.Body != "there" {
		t.Errorf("notification not preserved: %+v", envelope.Message.Notification)
	}
	if envelope.Message.Data.Kind != "alert" {
		t.Errorf("data.kind = %q, want alert", envelope.Message.Data.Kind)
	}
}

func TestClient_InjectsDistinctTokenPerRequest(t *testing.T) {
	t.Parallel()

	var (
		gotTokens []string
		mu        sync.Mutex
	)
	fcmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var envelope struct {
			Message struct {
				Token string `json:"token"`
			} `json:"message"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &envelope)
		mu.Lock()
		gotTokens = append(gotTokens, envelope.Message.Token)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer fcmSrv.Close()
	tokenSrv, _ := tokenServer(t)

	client := newTestClient(t, fcmSrv, tokenSrv)
	if _, err := client.Send(context.Background(), []string{"tok-a", "tok-b"}, json.RawMessage(`{"data":{}}`)); err != nil {
		t.Fatalf("Send: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(gotTokens) != 2 {
		t.Fatalf("requests = %d, want 2 (one per token)", len(gotTokens))
	}
	got := map[string]bool{gotTokens[0]: true, gotTokens[1]: true}
	if !got["tok-a"] || !got["tok-b"] {
		t.Errorf("tokens = %v, want tok-a and tok-b", gotTokens)
	}
}

func TestClient_ReportsUnregisteredTokenAsInvalid(t *testing.T) {
	t.Parallel()

	fcmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":404,"status":"NOT_FOUND","message":"Requested entity was not found.","details":[{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":"UNREGISTERED"}]}}`))
	}))
	defer fcmSrv.Close()
	tokenSrv, _ := tokenServer(t)

	client := newTestClient(t, fcmSrv, tokenSrv)
	invalid, err := client.Send(context.Background(), []string{"deadtoken"}, json.RawMessage(`{"data":{}}`))
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(invalid) != 1 || invalid[0] != "deadtoken" {
		t.Fatalf("invalid = %v, want [deadtoken]", invalid)
	}
}

func TestClient_ReportsInvalidArgumentAsInvalid(t *testing.T) {
	t.Parallel()

	fcmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":400,"status":"INVALID_ARGUMENT","message":"The registration token is not a valid FCM registration token","details":[{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":"INVALID_ARGUMENT"}]}}`))
	}))
	defer fcmSrv.Close()
	tokenSrv, _ := tokenServer(t)

	client := newTestClient(t, fcmSrv, tokenSrv)
	invalid, err := client.Send(context.Background(), []string{"badtoken"}, json.RawMessage(`{"data":{}}`))
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(invalid) != 1 || invalid[0] != "badtoken" {
		t.Fatalf("invalid = %v, want [badtoken]", invalid)
	}
}

func TestClient_TransientErrorIsSkippedNotPrunedAndNotRetried(t *testing.T) {
	t.Parallel()

	var (
		mu    sync.Mutex
		calls int
	)
	fcmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"code":503,"status":"UNAVAILABLE","message":"The service is currently unavailable."}}`))
	}))
	defer fcmSrv.Close()
	tokenSrv, _ := tokenServer(t)

	client := newTestClient(t, fcmSrv, tokenSrv)
	invalid, err := client.Send(context.Background(), []string{"tok"}, json.RawMessage(`{"data":{}}`))
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(invalid) != 0 {
		t.Fatalf("invalid = %v, want none (transient error must not prune)", invalid)
	}

	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Fatalf("calls = %d, want 1 (no retry loop day-1)", calls)
	}
}

func TestClient_CachesAccessTokenAcrossSends(t *testing.T) {
	t.Parallel()

	fcmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer fcmSrv.Close()
	tokenSrv, tokenCalls := tokenServer(t)

	client := newTestClient(t, fcmSrv, tokenSrv)
	for range 3 {
		if _, err := client.Send(context.Background(), []string{"tok"}, json.RawMessage(`{"data":{}}`)); err != nil {
			t.Fatalf("Send: %v", err)
		}
	}
	if tokenCalls() != 1 {
		t.Errorf("token endpoint calls = %d, want 1 (access token cached across sends)", tokenCalls())
	}
}

func TestClient_EmptyTokensIsNoop(t *testing.T) {
	t.Parallel()

	fcmSrv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("FCM server should not be called with no tokens")
	}))
	defer fcmSrv.Close()
	tokenSrv, tokenCalls := tokenServer(t)

	client := newTestClient(t, fcmSrv, tokenSrv)
	invalid, err := client.Send(context.Background(), nil, json.RawMessage(`{"data":{}}`))
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(invalid) != 0 {
		t.Fatalf("invalid = %v, want none", invalid)
	}
	if tokenCalls() != 0 {
		t.Errorf("token endpoint must not be hit for an empty send, got %d", tokenCalls())
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

func TestNewClient_DisabledOptionsValidateNoop(t *testing.T) {
	t.Parallel()
	// A disabled option set must not require project id / service account.
	if err := (Options{Enabled: false}).validate(); err != nil {
		t.Fatalf("disabled validate: %v", err)
	}
	if err := (Options{Enabled: true, ProjectID: "p"}).validate(); err == nil {
		t.Fatal("enabled without service account must fail validation")
	}
}
