package sharelink

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
)

func recordOnSpan(t *testing.T, r *http.Request) (string, sdktrace.ReadOnlySpan) {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	ctx, span := tp.Tracer("test").Start(r.Context(), "request")
	code := Record(r.WithContext(ctx))
	span.End()
	return code, rec.Ended()[0]
}

func nonceOn(span sdktrace.ReadOnlySpan) (string, bool) {
	for _, kv := range span.Attributes() {
		if kv.Key == "share.nonce" {
			return kv.Value.AsString(), true
		}
	}
	return "", false
}

func TestRecord_QueryValues(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		query string
		want  string
	}{
		{"valid", "?s=aZ09bY", "aZ09bY"},
		{"absent", "", ""},
		{"empty", "?s=", ""},
		{"too short", "?s=abc12", ""},
		{"too long", "?s=abc1234", ""},
		{"punctuation", "?s=abc-12", ""},
		{"markup", "?s=%3Cb%3E123", ""},
		{"trailing newline", "?s=abc123%0A", ""},
		{"non-ascii", "?s=abc12%C3%A9", ""},
		{"other parameter", "?t=abc123", ""},
		{"first value wins", "?s=bad&s=abc123", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, span := recordOnSpan(t, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/a/x/y"+tc.query, nil))
			if got != tc.want {
				t.Errorf("Record = %q, want %q", got, tc.want)
			}
			nonce, present := nonceOn(span)
			if present != (tc.want != "") || nonce != tc.want {
				t.Errorf("share.nonce = %q (present %v), want %q", nonce, present, tc.want)
			}
		})
	}
}

func TestRecord_NeverOnARequestCarryingAnAccount(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		prepare func(*http.Request) *http.Request
	}{
		{"bearer header on an anonymous route", func(r *http.Request) *http.Request {
			r.Header.Set("Authorization", "Bearer token")
			return r
		}},
		{"validated subject", func(r *http.Request) *http.Request {
			return r.WithContext(auth.WithSubject(context.Background(), "auth0|u"))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := tc.prepare(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/a/x/y?s=abc123", nil))
			got, span := recordOnSpan(t, r)
			if got != "" {
				t.Errorf("Record = %q, want empty", got)
			}
			if nonce, present := nonceOn(span); present {
				t.Errorf("share.nonce = %q, want absent", nonce)
			}
		})
	}
}

func TestRecord_LeavesNoRawQueryOnTheSpan(t *testing.T) {
	t.Parallel()
	_, span := recordOnSpan(t, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/a/x/y?s=abc123&secret=hunter2", nil))
	for _, kv := range span.Attributes() {
		if strings.Contains(kv.Value.String(), "hunter2") {
			t.Errorf("attribute %s carries the raw query", kv.Key)
		}
	}
}
