package apns

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

type failingTransport struct{ err error }

func (f failingTransport) RoundTrip(*http.Request) (*http.Response, error) { return nil, f.err }

const secretToken = "abcdef0123456789abcdef0123456789abcdef0123456789"

func newFailingClient(t *testing.T, transportErr error, logs *bytes.Buffer) *Client {
	t.Helper()
	pemBytes, _ := newTestKeyPEM(t)
	opts := Options{Enabled: true, AuthKey: string(pemBytes), KeyID: "L2J5PQASN5", TeamID: "4574VQ7N2X", BundleID: "uk.towncrierapp.mobile"}
	httpClient := &http.Client{Transport: failingTransport{err: transportErr}}
	logger := slog.New(slog.NewTextHandler(logs, nil))
	c, err := newClientWithBaseURL(opts, "https://apns.test", httpClient, logger, func() time.Time { return time.Unix(1_700_000_000, 0).UTC() })
	if err != nil {
		t.Fatalf("newClientWithBaseURL: %v", err)
	}
	return c
}

func TestClient_TransportErrorOmitsDeviceToken(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	client := newFailingClient(t, errors.New("connection refused"), &logs)

	_, err := client.do(context.Background(), secretToken, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("do: want error")
	}
	if strings.Contains(err.Error(), secretToken) {
		t.Fatalf("returned error text contains device token: %q", err.Error())
	}

	if _, sendErr := client.Send(context.Background(), []string{secretToken}, json.RawMessage(`{}`)); sendErr != nil {
		t.Fatalf("Send: %v", sendErr)
	}
	if strings.Contains(logs.String(), secretToken) {
		t.Fatalf("log output contains device token: %q", logs.String())
	}
}

func TestClient_TransportErrorOmitsDeviceTokenFromSpan(t *testing.T) {
	tp, rec := recorderProvider(t)
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	var logs bytes.Buffer
	client := newFailingClient(t, errors.New("connection refused"), &logs)
	if _, err := client.Send(context.Background(), []string{secretToken}, json.RawMessage(`{}`)); err != nil {
		t.Fatalf("Send: %v", err)
	}

	for _, s := range rec.Ended() {
		if strings.Contains(s.Status().Description, secretToken) {
			t.Fatalf("span %q status contains device token", s.Name())
		}
		if s.Status().Code != codes.Error {
			continue
		}
		for _, ev := range s.Events() {
			for _, a := range ev.Attributes {
				if strings.Contains(a.Value.String(), secretToken) {
					t.Fatalf("span %q event attribute %q contains device token", s.Name(), a.Key)
				}
			}
		}
	}
}

func TestClient_TransportErrorKeepsClassification(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		err  error
	}{
		{"deadline", context.DeadlineExceeded},
		{"canceled", context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var logs bytes.Buffer
			client := newFailingClient(t, tc.err, &logs)
			_, err := client.do(context.Background(), secretToken, json.RawMessage(`{}`))
			if !errors.Is(err, tc.err) {
				t.Fatalf("errors.Is(%v) = false; err = %v", tc.err, err)
			}
			var urlErr *url.Error
			if !errors.As(err, &urlErr) {
				t.Fatalf("errors.As(*url.Error) = false; err = %v", err)
			}
			if strings.Contains(urlErr.URL, secretToken) {
				t.Fatalf("url.Error URL contains device token: %q", urlErr.URL)
			}
		})
	}
}
