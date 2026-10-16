package subscriptionevents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

type fakeTotals struct {
	calls atomic.Int32
	total int
	cands []*profiles.UserProfile
	err   error
}

func (f *fakeTotals) UserStats(context.Context, time.Time) (profiles.UserStats, error) {
	f.calls.Add(1)
	return profiles.UserStats{Total: f.total}, f.err
}

func (f *fakeTotals) PaidCandidates(context.Context) ([]*profiles.UserProfile, error) {
	return f.cands, f.err
}

type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

type capturedRequest struct {
	header http.Header
	body   []byte
}

func captureServer(t *testing.T, status int, delay time.Duration) (*httptest.Server, func() []capturedRequest) {
	t.Helper()
	var mu sync.Mutex
	var got []capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		got = append(got, capturedRequest{header: r.Header.Clone(), body: b})
		mu.Unlock()
		time.Sleep(delay)
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv, func() []capturedRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]capturedRequest(nil), got...)
	}
}

var (
	snapFree = Snapshot{Tier: profiles.TierFree, Plan: PlanNone}
	snapPro  = Snapshot{Tier: profiles.TierPro, Plan: PlanAnnual}
)

func newTestNotifier(reader TotalsReader, hooks []Webhook, client *http.Client, logs *syncBuffer) *Notifier {
	logger := slog.New(slog.NewTextHandler(logs, nil))
	return NewNotifier(reader, hooks, client, logger, func() time.Time { return totalsNow })
}

func TestNotifier_RawBodyAndHeaders(t *testing.T) {
	t.Parallel()
	srv, requests := captureServer(t, 200, 0)
	reader := &fakeTotals{total: 5}
	n := newTestNotifier(reader, []Webhook{{URL: srv.URL, Headers: map[string]string{"Authorization": "Bearer abc", "X-Extra": "1"}}}, srv.Client(), &syncBuffer{})

	n.Changed(context.Background(), snapFree, snapPro, SourceOfferCode)
	n.Wait()

	reqs := requests()
	if len(reqs) != 1 {
		t.Fatalf("requests = %d, want 1", len(reqs))
	}
	if reqs[0].header.Get("Content-Type") != "application/json" || reqs[0].header.Get("Authorization") != "Bearer abc" || reqs[0].header.Get("X-Extra") != "1" {
		t.Errorf("headers = %v", reqs[0].header)
	}
	var p Payload
	if err := json.Unmarshal(reqs[0].body, &p); err != nil {
		t.Fatal(err)
	}
	if p.Event.Type != EventPaidStarted || p.Event.Source != SourceOfferCode || p.Totals.Users != 5 || p.EventID == "" {
		t.Errorf("payload = %+v", p)
	}
}

func TestNotifier_BodyFieldWrapsPayloadString(t *testing.T) {
	t.Parallel()
	srv, requests := captureServer(t, 200, 0)
	n := newTestNotifier(&fakeTotals{total: 1}, []Webhook{{URL: srv.URL, BodyField: "text"}}, srv.Client(), &syncBuffer{})

	n.Changed(context.Background(), snapFree, snapPro, SourceAdmin)
	n.Wait()

	reqs := requests()
	if len(reqs) != 1 {
		t.Fatalf("requests = %d", len(reqs))
	}
	var wrapper map[string]string
	if err := json.Unmarshal(reqs[0].body, &wrapper); err != nil || len(wrapper) != 1 {
		t.Fatalf("wrapper = %v, %v", wrapper, err)
	}
	var p Payload
	if err := json.Unmarshal([]byte(wrapper["text"]), &p); err != nil {
		t.Fatal(err)
	}
	if p.Event.Type != EventPaidStarted || p.Event.Source != SourceAdmin {
		t.Errorf("payload = %+v", p)
	}
}

func TestNotifier_FailureLogsWarningWithoutSecrets(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		status int
		delay  time.Duration
	}{
		{"server error", 500, 0},
		{"timeout", 200, 300 * time.Millisecond},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv, _ := captureServer(t, tc.status, tc.delay)
			client := &http.Client{Timeout: 50 * time.Millisecond}
			logs := &syncBuffer{}
			hook := Webhook{URL: srv.URL + "/secret-path", Headers: map[string]string{"Authorization": "Bearer SECRET-TOKEN"}}
			n := newTestNotifier(&fakeTotals{}, []Webhook{hook}, client, logs)

			n.Changed(context.Background(), snapPro, snapFree, SourceSweep)
			n.Wait()

			out := logs.String()
			if !containsAny(out, "level=WARN") || !containsAny(out, "webhook_index=0", "event_type=reverted_to_free") {
				t.Errorf("expected warning with index and event type, got %q", out)
			}
			if containsAny(out, "secret-path", "SECRET-TOKEN", "127.0.0.1") {
				t.Errorf("log leaks webhook config: %q", out)
			}
		})
	}
}

func TestNotifier_NoOps(t *testing.T) {
	t.Parallel()
	srv, requests := captureServer(t, 200, 0)
	hook := []Webhook{{URL: srv.URL}}

	t.Run("no event", func(t *testing.T) {
		t.Parallel()
		reader := &fakeTotals{}
		n := newTestNotifier(reader, hook, srv.Client(), &syncBuffer{})
		n.Changed(context.Background(), snapPro, snapPro, SourceAppStore)
		n.Wait()
		if reader.calls.Load() != 0 {
			t.Error("totals queried for a non-event")
		}
	})
	t.Run("zero webhooks", func(t *testing.T) {
		t.Parallel()
		reader := &fakeTotals{}
		n := newTestNotifier(reader, nil, srv.Client(), &syncBuffer{})
		n.Changed(context.Background(), snapFree, snapPro, SourceAppStore)
		n.Wait()
		if reader.calls.Load() != 0 {
			t.Error("totals queried with no webhooks")
		}
	})
	t.Run("nil notifier", func(t *testing.T) {
		t.Parallel()
		var n *Notifier
		n.Changed(context.Background(), snapFree, snapPro, SourceAppStore)
		n.Wait()
	})
	if len(requests()) != 0 {
		t.Error("unexpected delivery")
	}
}

func TestNotifier_TotalsErrorSkipsDelivery(t *testing.T) {
	t.Parallel()
	srv, requests := captureServer(t, 200, 0)
	logs := &syncBuffer{}
	n := newTestNotifier(&fakeTotals{err: errors.New("db down")}, []Webhook{{URL: srv.URL}}, srv.Client(), logs)
	n.Changed(context.Background(), snapFree, snapPro, SourceAppStore)
	n.Wait()
	if len(requests()) != 0 {
		t.Error("delivered despite totals failure")
	}
	if !containsAny(logs.String(), "level=WARN") {
		t.Errorf("logs = %q", logs.String())
	}
}

func TestNotifier_SurvivesCallerCancellation(t *testing.T) {
	t.Parallel()
	srv, requests := captureServer(t, 200, 0)
	n := newTestNotifier(&fakeTotals{}, []Webhook{{URL: srv.URL}}, srv.Client(), &syncBuffer{})
	ctx, cancel := context.WithCancel(context.Background())
	n.Changed(ctx, snapFree, snapPro, SourceAppStore)
	cancel()
	n.Wait()
	if len(requests()) != 1 {
		t.Errorf("requests = %d, want 1", len(requests()))
	}
}
