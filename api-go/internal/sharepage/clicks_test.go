package sharepage

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const browserUA = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 Version/18.0 Mobile Safari/604.1"

type recordedClick struct {
	slug   string
	ref    string
	source SourceGroup
}

type fakeClickRecorder struct {
	mu     sync.Mutex
	clicks []recordedClick
}

func (f *fakeClickRecorder) Add(slug, ref string, source SourceGroup) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clicks = append(f.clicks, recordedClick{slug: slug, ref: ref, source: source})
}

func humanHeaders() map[string]string {
	return map[string]string{"Sec-Fetch-Mode": "navigate", "User-Agent": browserUA}
}

func withHeader(headers map[string]string, key, value string) map[string]string {
	headers[key] = value
	return headers
}

func serveClick(t *testing.T, clicks clickRecorder, store appStore, method, path string, headers map[string]string, logger *slog.Logger) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	Routes(mux, store, &fakeResolver{slugs: map[string]int{"croydon": 165}}, clicks, logger)
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), method, path, nil)
	req.Header.Del("User-Agent")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	mux.ServeHTTP(rec, req)
	return rec
}

func TestServe_HumanVisit_CountedBySourceGroup(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		path    string
		referer string
		want    SourceGroup
	}{
		{name: "fbclid without referer", path: "/a/croydon/23/03456/FUL?fbclid=abc", want: SourceFacebook},
		{name: "fbclid beats seo referer", path: "/a/croydon/23/03456/FUL?fbclid=abc", referer: "https://towncrierapp.uk/planning/croydon", want: SourceFacebook},
		{name: "empty fbclid", path: "/a/croydon/23/03456/FUL?fbclid=", want: SourceFacebook},
		{name: "facebook.com", path: "/a/croydon/23/03456/FUL", referer: "https://facebook.com/", want: SourceFacebook},
		{name: "subdomain of facebook.com", path: "/a/croydon/23/03456/FUL", referer: "https://l.facebook.com/l.php?u=x", want: SourceFacebook},
		{name: "fb.com", path: "/a/croydon/23/03456/FUL", referer: "https://fb.com/", want: SourceFacebook},
		{name: "uppercase host", path: "/a/croydon/23/03456/FUL", referer: "https://M.FACEBOOK.COM/", want: SourceFacebook},
		{name: "apex seo", path: "/a/croydon/23/03456/FUL", referer: "https://towncrierapp.uk/planning/croydon", want: SourceSEO},
		{name: "subdomain seo", path: "/a/croydon/23/03456/FUL", referer: "https://share.towncrierapp.uk/a/croydon/1", want: SourceSEO},
		{name: "other referer", path: "/a/croydon/23/03456/FUL", referer: "https://www.google.com/", want: SourceOther},
		{name: "lookalike facebook host", path: "/a/croydon/23/03456/FUL", referer: "https://notfacebook.com/", want: SourceOther},
		{name: "lookalike seo host", path: "/a/croydon/23/03456/FUL", referer: "https://towncrierapp.uk.evil.example/", want: SourceOther},
		{name: "facebook in referer path only", path: "/a/croydon/23/03456/FUL", referer: "https://example.com/facebook.com", want: SourceOther},
		{name: "unparseable referer", path: "/a/croydon/23/03456/FUL", referer: "http://[bad", want: SourceOther},
		{name: "no referer", path: "/a/croydon/23/03456/FUL", want: SourceNone},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clicks := &fakeClickRecorder{}
			headers := humanHeaders()
			if tc.referer != "" {
				headers["Referer"] = tc.referer
			}
			rec := serveClick(t, clicks, &fakeStore{app: fullApp(t), found: true}, http.MethodGet, tc.path, headers, slog.New(slog.DiscardHandler))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d", rec.Code)
			}
			if len(clicks.clicks) != 1 {
				t.Fatalf("clicks = %+v, want exactly one", clicks.clicks)
			}
			want := recordedClick{slug: "croydon", ref: "23/03456/FUL", source: tc.want}
			if clicks.clicks[0] != want {
				t.Fatalf("click = %+v, want %+v", clicks.clicks[0], want)
			}
		})
	}
}

func TestServe_NotLikelyHuman_NotCounted(t *testing.T) {
	t.Parallel()
	agents := []string{
		"facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)",
		"Facebot",
		"WhatsApp/2.23.20.0",
		"Twitterbot/1.0",
		"Slackbot-LinkExpanding 1.0 (+https://api.slack.com/robots)",
		"TelegramBot (like TwitterBot)",
		"Mozilla/5.0 (compatible; Discordbot/2.0; +https://discordapp.com)",
		"LinkedInBot/1.0 (compatible; Mozilla/5.0)",
		"Mozilla/5.0 (Macintosh) Applebot/0.1",
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		"Mozilla/5.0 (compatible; bingbot/2.0)",
		"Mozilla/5.0 (Linux; Android 5.0) Bytespider",
		"Mozilla/5.0 AppleWebKit/537.36; compatible; GPTBot/1.0",
		"Mozilla/5.0 AppleWebKit/537.36; compatible; ClaudeBot/1.0",
		"SomeRandomBOT/3.0",
		"acme-crawler/1.0",
		"AcmeSpider",
	}
	for _, agent := range agents {
		t.Run(agent, func(t *testing.T) {
			t.Parallel()
			clicks := &fakeClickRecorder{}
			rec := serveClick(t, clicks, &fakeStore{app: fullApp(t), found: true}, http.MethodGet, "/a/croydon/23/03456/FUL", withHeader(humanHeaders(), "User-Agent", agent), slog.New(slog.DiscardHandler))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d", rec.Code)
			}
			if len(clicks.clicks) != 0 {
				t.Fatalf("clicks = %+v, want none", clicks.clicks)
			}
		})
	}
}

func TestServe_SecFetchModeNotNavigate_NotCounted(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		headers map[string]string
	}{
		{name: "missing", headers: map[string]string{"User-Agent": browserUA}},
		{name: "cors", headers: withHeader(humanHeaders(), "Sec-Fetch-Mode", "cors")},
		{name: "no-cors", headers: withHeader(humanHeaders(), "Sec-Fetch-Mode", "no-cors")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clicks := &fakeClickRecorder{}
			serveClick(t, clicks, &fakeStore{app: fullApp(t), found: true}, http.MethodGet, "/a/croydon/23/03456/FUL", tc.headers, slog.New(slog.DiscardHandler))
			if len(clicks.clicks) != 0 {
				t.Fatalf("clicks = %+v, want none", clicks.clicks)
			}
		})
	}
}

func TestServe_NonRenderOutcomes_NotCounted(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		method string
		path   string
		store  func(t *testing.T) *fakeStore
		want   int
	}{
		{name: "unknown ref 404", method: http.MethodGet, path: "/a/croydon/nope", store: func(*testing.T) *fakeStore { return &fakeStore{} }, want: http.StatusNotFound},
		{name: "unknown slug 404", method: http.MethodGet, path: "/a/unknown/23/03456/FUL", store: func(*testing.T) *fakeStore { return &fakeStore{} }, want: http.StatusNotFound},
		{name: "store error 500", method: http.MethodGet, path: "/a/croydon/23/03456/FUL", store: func(*testing.T) *fakeStore { return &fakeStore{err: fmt.Errorf("boom")} }, want: http.StatusInternalServerError},
		{name: "HEAD", method: http.MethodHead, path: "/a/croydon/23/03456/FUL", store: func(t *testing.T) *fakeStore { t.Helper(); return &fakeStore{app: fullApp(t), found: true} }, want: http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clicks := &fakeClickRecorder{}
			rec := serveClick(t, clicks, tc.store(t), tc.method, tc.path, humanHeaders(), slog.New(slog.DiscardHandler))
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
			if len(clicks.clicks) != 0 {
				t.Fatalf("clicks = %+v, want none", clicks.clicks)
			}
		})
	}
}

func TestServe_NilClickRecorder_StillRenders(t *testing.T) {
	t.Parallel()
	rec := serveClick(t, nil, &fakeStore{app: fullApp(t), found: true}, http.MethodGet, "/a/croydon/23/03456/FUL", humanHeaders(), slog.New(slog.DiscardHandler))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestServe_CountedVisit_ResponseIdenticalToUncounted(t *testing.T) {
	t.Parallel()
	store := &fakeStore{app: fullApp(t), found: true}
	logger := slog.New(slog.DiscardHandler)
	counted := serveClick(t, &fakeClickRecorder{}, store, http.MethodGet, "/a/croydon/23/03456/FUL", humanHeaders(), logger)
	plain := serveClick(t, nil, store, http.MethodGet, "/a/croydon/23/03456/FUL", nil, logger)
	if !bytes.Equal(counted.Body.Bytes(), plain.Body.Bytes()) {
		t.Fatal("counted page body differs from the uncounted page body")
	}
	if got, want := counted.Header().Get("Cache-Control"), "public, max-age=3600"; got != want {
		t.Fatalf("Cache-Control = %q, want %q", got, want)
	}
	if got := counted.Header().Get("Set-Cookie"); got != "" {
		t.Fatalf("Set-Cookie = %q, want none", got)
	}
}

func TestServe_NothingSensitivePersistedOrLogged(t *testing.T) {
	t.Parallel()
	writer := &fakeClickWriter{}
	logs := &capturingHandler{}
	logger := slog.New(logs)
	counter := NewClickCounter(writer, fixedNow, logger)

	const (
		fbclid  = "FBCLIDSECRET"
		referer = "https://l.facebook.com/REFERERSECRET?u=1"
		agent   = "Mozilla/5.0 AGENTSECRET"
		cookie  = "COOKIESECRET"
		ip      = "203.0.113.77"
	)
	headers := withHeader(humanHeaders(), "User-Agent", agent)
	headers["Referer"] = referer
	headers["Cookie"] = "sid=" + cookie
	headers["CF-Connecting-IP"] = ip
	serveClick(t, counter, &fakeStore{app: fullApp(t), found: true}, http.MethodGet, "/a/croydon/23/03456/FUL?fbclid="+fbclid, headers, logger)
	counter.flush(context.Background())

	rows := writer.allRows()
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want one", rows)
	}
	want := ClickTotal{Day: fixedDay, AuthoritySlug: "croydon", Ref: "23/03456/FUL", Source: SourceFacebook, Count: 1}
	if rows[0] != want {
		t.Fatalf("row = %+v, want %+v", rows[0], want)
	}

	var logged strings.Builder
	for _, r := range logs.records {
		fmt.Fprintf(&logged, "%s ", r.Message)
		r.Attrs(func(a slog.Attr) bool {
			fmt.Fprintf(&logged, "%s=%v ", a.Key, a.Value)
			return true
		})
	}
	persisted := fmt.Sprintf("%+v", rows)
	for _, secret := range []string{fbclid, "REFERERSECRET", "AGENTSECRET", cookie, ip, "navigate"} {
		if strings.Contains(persisted, secret) {
			t.Errorf("persisted row contains %q", secret)
		}
		if strings.Contains(logged.String(), secret) {
			t.Errorf("logs contain %q", secret)
		}
	}
}
