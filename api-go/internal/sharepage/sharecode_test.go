package sharepage

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

const (
	sharePath    = "/a/croydon/23/03456/FUL"
	canonicalTag = `rel="canonical" href="https://share.towncrierapp.uk/a/croydon/23/03456/FUL"`
)

func serveTraced(t *testing.T, path string, configure func(*http.Request)) (*httptest.ResponseRecorder, sdktrace.ReadOnlySpan) {
	t.Helper()
	mux := http.NewServeMux()
	store := &fakeStore{app: fullApp(t), found: true}
	Routes(mux, store, &fakeResolver{slugs: map[string]int{"croydon": 165}}, nil, slog.New(slog.DiscardHandler))

	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	if configure != nil {
		configure(req)
	}
	ctx, span := tp.Tracer("test").Start(req.Context(), "request")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req.WithContext(ctx))
	span.End()
	return rec, recorder.Ended()[0]
}

func nonceOn(span sdktrace.ReadOnlySpan) (string, bool) {
	for _, kv := range span.Attributes() {
		if kv.Key == "share.nonce" {
			return kv.Value.AsString(), true
		}
	}
	return "", false
}

func TestServe_ValidCode_MarksSpanAndKeepsItOnOGURLAndCTAs(t *testing.T) {
	t.Parallel()
	rec, span := serveTraced(t, sharePath+"?s=aZ09bY", nil)
	body := rec.Body.String()

	if nonce, _ := nonceOn(span); nonce != "aZ09bY" {
		t.Errorf("share.nonce = %q", nonce)
	}
	if !strings.Contains(body, `property="og:url" content="https://share.towncrierapp.uk/a/croydon/23/03456/FUL?s=aZ09bY"`) {
		t.Error("og:url must carry the code")
	}
	if !strings.Contains(body, canonicalTag) {
		t.Error("canonical must stay clean")
	}
	if got := strings.Count(body, `href="/go/app-store?s=aZ09bY"`); got != 2 {
		t.Errorf("CTA links with the code = %d, want 2", got)
	}
	if strings.Contains(body, "apps.apple.com") {
		t.Error("the page must link to the first-party redirect, not the App Store")
	}
	if got := strings.Count(body, "aZ09bY"); got != 3 {
		t.Errorf("code appears %d times, want 3 (og:url and two CTAs)", got)
	}
}

func TestServe_InvalidOrAbsentCode_RendersThePlainPage(t *testing.T) {
	t.Parallel()
	plain, _ := serveTraced(t, sharePath, nil)
	if got := strings.Count(plain.Body.String(), `href="/go/app-store"`); got != 2 {
		t.Errorf("plain CTA links = %d, want 2", got)
	}

	for _, query := range []string{"?s=abc", "?s=%22%3E%3Cscript%3E", "?s=abc1234", "?s=abc-12"} {
		t.Run(query, func(t *testing.T) {
			t.Parallel()
			rec, span := serveTraced(t, sharePath+query, nil)
			if rec.Body.String() != plain.Body.String() {
				t.Error("an invalid code must leave the page byte-identical to the plain page")
			}
			if _, present := nonceOn(span); present {
				t.Error("an invalid code must not reach the span")
			}
		})
	}
}

func TestServe_RequestCarryingAnAccount_RecordsNoCode(t *testing.T) {
	t.Parallel()
	rec, span := serveTraced(t, sharePath+"?s=abc123", func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer token")
	})
	if _, present := nonceOn(span); present {
		t.Error("share.nonce must not coexist with an account")
	}
	if strings.Contains(rec.Body.String(), "abc123") {
		t.Error("the page must not carry the code")
	}
}

func TestAppStoreRedirect(t *testing.T) {
	t.Parallel()
	const target = "https://apps.apple.com/gb/app/town-crier-planning-alerts/id6764095657?pt=128810278&ct=share-page&mt=8"
	for _, tc := range []struct {
		name      string
		path      string
		wantNonce string
	}{
		{"no code", "/go/app-store", ""},
		{"valid code", "/go/app-store?s=abc123", "abc123"},
		{"invalid code", "/go/app-store?s=abc", ""},
		{"destination parameters are ignored", "/go/app-store?url=https://evil.example&to=https://evil.example&next=//evil.example", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec, span := serveTraced(t, tc.path, nil)
			if rec.Code != http.StatusFound {
				t.Fatalf("status = %d, want 302", rec.Code)
			}
			if got := rec.Header().Get("Location"); got != target {
				t.Errorf("Location = %q", got)
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q", got)
			}
			nonce, _ := nonceOn(span)
			if nonce != tc.wantNonce {
				t.Errorf("share.nonce = %q, want %q", nonce, tc.wantNonce)
			}
		})
	}
}
