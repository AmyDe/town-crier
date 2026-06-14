package acsemail

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// newTestClient builds a Client signing against the httptest server's host but
// targeting its URL. The poll interval is squeezed to keep tests fast.
func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	creds := credentials{
		endpoint:  srv.URL,
		accessKey: newSecret("dGVzdC1zaWduaW5nLWtleQ=="),
	}
	client := newClientWithCreds(creds, srv.Client(), testLogger(), func() time.Time {
		return time.Unix(1_700_000_000, 0).UTC()
	})
	client.pollInterval = time.Millisecond
	client.maxPolls = 5
	return client
}

func testMessage() Message {
	return Message{
		Sender:    "hello@towncrierapp.uk",
		Recipient: "user@example.com",
		Subject:   "Planning update",
		HTMLBody:  "<p>hi</p>",
	}
}

func TestClient_SendSignsAndPostsToEmailsSend(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	var srvURL string
	mux.HandleFunc("POST /emails:send", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		if got := r.URL.RawQuery; got != "api-version=2023-03-31" {
			http.Error(w, "bad api-version: "+got, http.StatusBadRequest)
			return
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "HMAC-SHA256 SignedHeaders=x-ms-date;host;x-ms-content-sha256&Signature=") {
			http.Error(w, "bad auth: "+auth, http.StatusUnauthorized)
			return
		}
		if r.Header.Get("x-ms-date") == "" {
			http.Error(w, "missing x-ms-date", http.StatusBadRequest)
			return
		}
		if r.Header.Get("x-ms-content-sha256") != computeContentHash(body) {
			http.Error(w, "content hash mismatch", http.StatusBadRequest)
			return
		}
		var msg sendRequest
		if err := json.Unmarshal(body, &msg); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if msg.SenderAddress == "" || len(msg.Recipients.To) == 0 || msg.Content.HTML == "" {
			http.Error(w, "incomplete message", http.StatusBadRequest)
			return
		}

		w.Header().Set("Operation-Location", srvURL+"/emails/operations/op-1?api-version=2023-03-31")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"id":"op-1","status":"Running"}`))
	})
	mux.HandleFunc("GET /emails/operations/op-1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"op-1","status":"Succeeded"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	srvURL = srv.URL

	client := newTestClient(t, srv)
	if err := client.Send(context.Background(), testMessage()); err != nil {
		t.Fatalf("Send: %v", err)
	}
}

func TestClient_PollsOperationUntilSucceeded(t *testing.T) {
	t.Parallel()

	var (
		mu        sync.Mutex
		pollsSeen int
	)
	mux := http.NewServeMux()
	var srvURL string
	mux.HandleFunc("POST /emails:send", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Operation-Location", srvURL+"/emails/operations/op-2?api-version=2023-03-31")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"id":"op-2","status":"Running"}`))
	})
	mux.HandleFunc("GET /emails/operations/op-2", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		pollsSeen++
		n := pollsSeen
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		if n < 3 {
			_, _ = w.Write([]byte(`{"id":"op-2","status":"Running"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"op-2","status":"Succeeded"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	srvURL = srv.URL

	client := newTestClient(t, srv)
	if err := client.Send(context.Background(), testMessage()); err != nil {
		t.Fatalf("Send: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if pollsSeen < 3 {
		t.Fatalf("pollsSeen = %d, want >= 3 (polled until Succeeded)", pollsSeen)
	}
}

func TestClient_ReturnsErrorOnFailedOperation(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	var srvURL string
	mux.HandleFunc("POST /emails:send", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Operation-Location", srvURL+"/emails/operations/op-3?api-version=2023-03-31")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"id":"op-3","status":"Running"}`))
	})
	mux.HandleFunc("GET /emails/operations/op-3", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"op-3","status":"Failed","error":{"message":"recipient rejected"}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	srvURL = srv.URL

	client := newTestClient(t, srv)
	err := client.Send(context.Background(), testMessage())
	if err == nil {
		t.Fatal("expected an error for a Failed operation, got nil")
	}
}

func TestClient_ReturnsErrorOnSendRejection(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		http.Error(w, `{"error":{"message":"bad request"}}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	err := client.Send(context.Background(), testMessage())
	if err == nil {
		t.Fatal("expected an error for a 400 send response, got nil")
	}
}

func TestNoOpSender_SendIsNoError(t *testing.T) {
	t.Parallel()

	sender := NewNoOpSender()
	if err := sender.Send(context.Background(), testMessage()); err != nil {
		t.Fatalf("Send: %v", err)
	}
}
