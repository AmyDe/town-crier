package acsemail

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
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
		accessKey: platform.NewSecret("dGVzdC1zaWduaW5nLWtleQ=="),
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

var guidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func TestClient_SendsRepeatabilityHeaders(t *testing.T) {
	t.Parallel()

	var (
		mu      sync.Mutex
		ids     []string
		firstAt []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		mu.Lock()
		ids = append(ids, r.Header.Get("Repeatability-Request-Id"))
		firstAt = append(firstAt, r.Header.Get("Repeatability-First-Sent"))
		mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"id":"op","status":"Succeeded"}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	msg := testMessage()
	msg.RepeatabilityKey = "user-1|hourly:2026-10-24T10"
	other := msg
	other.RepeatabilityKey = "user-1|hourly:2026-10-24T11"
	for _, m := range []Message{msg, msg, other} {
		if err := client.Send(context.Background(), m); err != nil {
			t.Fatalf("Send: %v", err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if !guidPattern.MatchString(ids[0]) {
		t.Fatalf("Repeatability-Request-Id = %q, want lowercase GUID", ids[0])
	}
	if ids[0] != ids[1] {
		t.Errorf("same key gave different ids: %q vs %q", ids[0], ids[1])
	}
	if ids[0] == ids[2] {
		t.Errorf("different keys gave the same id %q", ids[0])
	}
	want := time.Unix(1_700_000_000, 0).UTC().Format(http.TimeFormat)
	if firstAt[0] != want {
		t.Errorf("Repeatability-First-Sent = %q, want %q", firstAt[0], want)
	}
}

func TestClient_OmitsRepeatabilityHeadersWithoutKey(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		if r.Header.Get("Repeatability-Request-Id") != "" || r.Header.Get("Repeatability-First-Sent") != "" {
			http.Error(w, "unexpected repeatability headers", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"id":"op","status":"Succeeded"}`))
	}))
	defer srv.Close()

	if err := newTestClient(t, srv).Send(context.Background(), testMessage()); err != nil {
		t.Fatalf("Send: %v", err)
	}
}

func TestClient_AcceptedThenPollTimeoutIsStatusUnknown(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	var srvURL string
	mux.HandleFunc("POST /emails:send", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Operation-Location", srvURL+"/emails/operations/op-4?api-version=2023-03-31")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"id":"op-4","status":"Running"}`))
	})
	mux.HandleFunc("GET /emails/operations/op-4", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"op-4","status":"Running"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	srvURL = srv.URL

	err := newTestClient(t, srv).Send(context.Background(), testMessage())
	if !errors.Is(err, ErrAcceptedStatusUnknown) {
		t.Fatalf("err = %v, want ErrAcceptedStatusUnknown", err)
	}
	if errors.Is(err, ErrSendFailed) {
		t.Errorf("status-unknown must not read as ErrSendFailed: %v", err)
	}
}

func TestClient_AcceptedThenPollErrorIsStatusUnknown(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	var srvURL string
	mux.HandleFunc("POST /emails:send", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Operation-Location", srvURL+"/emails/operations/op-5?api-version=2023-03-31")
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("GET /emails/operations/op-5", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	srvURL = srv.URL

	err := newTestClient(t, srv).Send(context.Background(), testMessage())
	if !errors.Is(err, ErrAcceptedStatusUnknown) {
		t.Fatalf("err = %v, want ErrAcceptedStatusUnknown", err)
	}
}

func TestClient_ConflictOnSendIsStatusUnknown(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		http.Error(w, "in progress", http.StatusConflict)
	}))
	defer srv.Close()

	err := newTestClient(t, srv).Send(context.Background(), testMessage())
	if !errors.Is(err, ErrAcceptedStatusUnknown) {
		t.Fatalf("err = %v, want ErrAcceptedStatusUnknown", err)
	}
}

// pollServer serves a 202 POST and delegates each status GET to poll (1-based
// call number). It returns the server and a pointer to the GET count.
func pollServer(t *testing.T, poll func(n int, w http.ResponseWriter)) (*httptest.Server, *int) {
	t.Helper()
	var mu sync.Mutex
	gets := 0
	mux := http.NewServeMux()
	var srvURL string
	mux.HandleFunc("POST /emails:send", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Operation-Location", srvURL+"/emails/operations/op?api-version=2023-03-31")
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("GET /emails/operations/op", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gets++
		n := gets
		mu.Unlock()
		poll(n, w)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	srvURL = srv.URL
	return srv, &gets
}

func TestSend_PollThrottledThenSucceeds(t *testing.T) {
	t.Parallel()
	srv, gets := pollServer(t, func(n int, w http.ResponseWriter) {
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"id":"op","status":"Succeeded"}`))
	})

	if err := newTestClient(t, srv).Send(context.Background(), testMessage()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if *gets != 2 {
		t.Errorf("GETs = %d, want 2", *gets)
	}
}

func TestSend_PollThrottledWithoutRetryAfterUsesPollInterval(t *testing.T) {
	t.Parallel()
	srv, _ := pollServer(t, func(n int, w http.ResponseWriter) {
		if n == 1 {
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"id":"op","status":"Succeeded"}`))
	})

	if err := newTestClient(t, srv).Send(context.Background(), testMessage()); err != nil {
		t.Fatalf("Send: %v", err)
	}
}

func TestSend_PollThrottledUntilBudgetExhausted(t *testing.T) {
	t.Parallel()
	srv, gets := pollServer(t, func(_ int, w http.ResponseWriter) {
		http.Error(w, "slow down", http.StatusTooManyRequests)
	})
	client := newTestClient(t, srv)

	err := client.Send(context.Background(), testMessage())
	if !errors.Is(err, ErrAcceptedStatusUnknown) {
		t.Fatalf("err = %v, want ErrAcceptedStatusUnknown", err)
	}
	if *gets != client.maxPolls {
		t.Errorf("GETs = %d, want %d", *gets, client.maxPolls)
	}
}

func TestSend_PollNon429ErrorStillFailsFast(t *testing.T) {
	t.Parallel()
	srv, gets := pollServer(t, func(_ int, w http.ResponseWriter) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})

	err := newTestClient(t, srv).Send(context.Background(), testMessage())
	if !errors.Is(err, ErrAcceptedStatusUnknown) {
		t.Fatalf("err = %v, want ErrAcceptedStatusUnknown", err)
	}
	if *gets != 1 {
		t.Errorf("GETs = %d, want 1", *gets)
	}
}

func TestSend_Post429IsRateLimited(t *testing.T) {
	t.Parallel()
	var gets int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		if r.Method == http.MethodGet {
			mu.Lock()
			gets++
			mu.Unlock()
		}
		http.Error(w, "quota", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	err := newTestClient(t, srv).Send(context.Background(), testMessage())
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("err = %v, want ErrRateLimited", err)
	}
	if !errors.Is(err, ErrSendFailed) {
		t.Errorf("err = %v, want ErrSendFailed", err)
	}
	if gets != 0 {
		t.Errorf("GETs = %d, want 0", gets)
	}
}

func TestThrottleWait(t *testing.T) {
	t.Parallel()
	const interval = 2 * time.Second
	tests := []struct {
		header string
		want   time.Duration
	}{
		{"5", 5 * time.Second},
		{"120", maxPollRetryAfter},
		{"", interval},
		{"abc", interval},
		{"-1", interval},
	}
	for _, tt := range tests {
		t.Run("header="+tt.header, func(t *testing.T) {
			t.Parallel()
			if got := throttleWait(tt.header, interval); got != tt.want {
				t.Errorf("throttleWait(%q) = %v, want %v", tt.header, got, tt.want)
			}
		})
	}
}
