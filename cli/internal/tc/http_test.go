package tc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func clientFor(server *httptest.Server) *Client {
	return NewClient(Config{URL: server.URL, APIKey: "sk-test"})
}

// testStatsJSON is a minimal but complete GET /v1/admin/stats body, shared by
// the stats render tests and the list-users summary-header tests.
const testStatsJSON = `{"users":{"total":2,"byTier":{"Free":1,"Personal":0,"Pro":1}},` +
	`"paying":{"effectivePaid":1,"appStore":1,"comped":0,"lapsed":0,"inGrace":0},` +
	`"signups":{"last24h":0,"last7d":1,"last30d":2,"mostRecent":{"userId":"u1","email":"a@b.com","createdAt":"2026-07-01T09:00:00Z"}},` +
	`"activity":{"active24h":1,"active7d":2,"zeroWatchZones":0,"noEmail":1},` +
	`"reach":{"watchZones":3,"savedApplications":5,"deviceRegistrations":2,"notificationsSent":10,"notificationsUnread":4}}`

func TestRunGenerateOfferCodes_StreamsCodesOnSuccess(t *testing.T) {
	t.Parallel()
	var gotBody generateOfferCodesRequest
	var gotKey, gotPath, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get(apiKeyHeader)
		gotPath = r.URL.Path
		gotMethod = r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "AAAA-BBBB\nCCCC-DDDD\n")
	}))
	defer server.Close()

	env, out, errBuf := captureEnv()
	code := runGenerateOfferCodes(context.Background(), clientFor(server), env, ParseArgs([]string{
		"generate-offer-codes", "--count", "2", "--tier", "Pro", "--duration-days", "30", "--label", "creator-campaign",
	}))

	if code != exitOK {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, errBuf.String())
	}
	if gotMethod != http.MethodPost || gotPath != "/v1/admin/offer-codes" {
		t.Fatalf("request = %s %s, want POST /v1/admin/offer-codes", gotMethod, gotPath)
	}
	if gotKey != "sk-test" {
		t.Fatalf("X-Admin-Key = %q, want sk-test", gotKey)
	}
	// MaxRedemptions omitted (--max-redemptions not given), so the JSON field is
	// absent and decodes back to a nil pointer.
	if gotBody.Count != 2 || gotBody.Tier != "Pro" || gotBody.DurationDays != 30 ||
		gotBody.Label != "creator-campaign" || gotBody.MaxRedemptions != nil {
		t.Fatalf("request body = %+v, want {2 Pro 30 creator-campaign <nil>}", gotBody)
	}
	if got := out.String(); got != "AAAA-BBBB\nCCCC-DDDD\n" {
		t.Fatalf("stdout = %q, want the two codes", got)
	}
	if !strings.Contains(errBuf.String(), `Generated 2 codes: Pro tier, 30 days duration, label "creator-campaign", max 1 redemptions`) {
		t.Fatalf("stderr = %q, want summary line", errBuf.String())
	}
}

// TestRunGenerateOfferCodes_PassesMaxRedemptions confirms --max-redemptions is
// forwarded as an explicit JSON field (not the omitted/default path).
func TestRunGenerateOfferCodes_PassesMaxRedemptions(t *testing.T) {
	t.Parallel()
	var gotBody generateOfferCodesRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "AAAA-BBBB\n")
	}))
	defer server.Close()

	env, _, errBuf := captureEnv()
	code := runGenerateOfferCodes(context.Background(), clientFor(server), env, ParseArgs([]string{
		"generate-offer-codes", "--count", "1", "--tier", "Pro", "--duration-days", "30",
		"--label", "creator-campaign", "--max-redemptions", "5",
	}))
	if code != exitOK {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, errBuf.String())
	}
	if gotBody.MaxRedemptions == nil || *gotBody.MaxRedemptions != 5 {
		t.Fatalf("request body MaxRedemptions = %v, want pointer to 5", gotBody.MaxRedemptions)
	}
	if !strings.Contains(errBuf.String(), "max 5 redemptions") {
		t.Fatalf("stderr = %q, want max 5 redemptions", errBuf.String())
	}
}

func TestRunGenerateOfferCodes_APIErrorReturns2(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":"bad"}`)
	}))
	defer server.Close()

	env, _, errBuf := captureEnv()
	code := runGenerateOfferCodes(context.Background(), clientFor(server), env, ParseArgs([]string{
		"generate-offer-codes", "--count", "2", "--tier", "Pro", "--duration-days", "30", "--label", "creator-campaign",
	}))

	if code != exitRuntime {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), `API error (400): {"error":"bad"}`) {
		t.Fatalf("stderr = %q, want API error (400)", errBuf.String())
	}
}

func TestRunGrantSubscription_SuccessReturns0(t *testing.T) {
	t.Parallel()
	var gotBody grantSubscriptionRequest
	var gotPath, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"userId":"u1","email":"a@b.com","tier":"Pro"}`)
	}))
	defer server.Close()

	env, out, _ := captureEnv()
	code := runGrantSubscription(context.Background(), clientFor(server), env, ParseArgs([]string{
		"grant-subscription", "--email", "a@b.com", "--tier", "pro",
	}))

	if code != exitOK {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if gotMethod != http.MethodPut || gotPath != "/v1/admin/subscriptions" {
		t.Fatalf("request = %s %s, want PUT /v1/admin/subscriptions", gotMethod, gotPath)
	}
	if gotBody != (grantSubscriptionRequest{Email: "a@b.com", Tier: "Pro"}) {
		t.Fatalf("request body = %+v, want {a@b.com Pro}", gotBody)
	}
	if got := out.String(); got != "Subscription granted: a@b.com -> Pro\n" {
		t.Fatalf("stdout = %q, want granted line", got)
	}
}

func TestRunGrantSubscription_NotFoundReturns2(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	env, _, errBuf := captureEnv()
	code := runGrantSubscription(context.Background(), clientFor(server), env, ParseArgs([]string{
		"grant-subscription", "--email", "missing@b.com", "--tier", "Pro",
	}))

	if code != exitRuntime {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "User not found: missing@b.com") {
		t.Fatalf("stderr = %q, want not-found message", errBuf.String())
	}
}

func TestRunGrantSubscription_InvalidTierReturns1(t *testing.T) {
	t.Parallel()
	env, _, errBuf := captureEnv()
	code := runGrantSubscription(context.Background(), dummyClient(), env, ParseArgs([]string{
		"grant-subscription", "--email", "a@b.com", "--tier", "Platinum",
	}))
	if code != exitUsage {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "Invalid tier: Platinum. Must be one of: Free, Personal, Pro") {
		t.Fatalf("stderr = %q, want invalid tier message", errBuf.String())
	}
}

func TestRunListUsers_SinglePageTable(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/admin/stats" {
			_, _ = io.WriteString(w, testStatsJSON)
			return
		}
		_, _ = io.WriteString(w, `{"items":[{"userId":"u1","email":"a@b.com","tier":"Pro"},{"userId":"u2","email":null,"tier":"Free"}],"continuationToken":null}`)
	}))
	defer server.Close()

	env, out, _ := captureEnv()
	code := runListUsers(context.Background(), clientFor(server), env, ParseArgs([]string{"list-users"}))
	if code != exitOK {
		t.Fatalf("exit code = %d, want 0", code)
	}

	got := out.String()
	for _, want := range []string{
		"UserId", "Email", "Tier", "WatchZones", "LastActive", "Created", "Notifs",
		"u1", "a@b.com", "u2", "(none)", "Free",
		"-", // legacy rows with no watch-zone / dates render "-"
		"0/0",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q; got:\n%s", want, got)
		}
	}
}

func TestRunListUsers_PaginatesOnYes(t *testing.T) {
	t.Parallel()
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/admin/stats" {
			_, _ = io.WriteString(w, testStatsJSON)
			return
		}
		paths = append(paths, r.URL.String())
		if r.URL.Query().Get("continuationToken") == "" {
			_, _ = io.WriteString(w, `{"items":[{"userId":"u1","email":"a@b.com","tier":"Pro"}],"continuationToken":"TOKEN1"}`)
			return
		}
		_, _ = io.WriteString(w, `{"items":[{"userId":"u2","email":"c@d.com","tier":"Free"}],"continuationToken":null}`)
	}))
	defer server.Close()

	env := Env{In: strings.NewReader("y\n"), Out: io.Discard, Err: io.Discard}
	code := runListUsers(context.Background(), clientFor(server), env, ParseArgs([]string{"list-users", "--page-size", "1"}))
	if code != exitOK {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if len(paths) != 2 {
		t.Fatalf("requests = %d (%v), want 2", len(paths), paths)
	}
	if !strings.Contains(paths[0], "pageSize=1") {
		t.Fatalf("first path = %q, want pageSize=1", paths[0])
	}
	if !strings.Contains(paths[1], "continuationToken=TOKEN1") {
		t.Fatalf("second path = %q, want continuationToken=TOKEN1", paths[1])
	}
}

func TestRunListUsers_StopsOnNo(t *testing.T) {
	t.Parallel()
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/admin/stats" {
			_, _ = io.WriteString(w, testStatsJSON)
			return
		}
		requests++
		_, _ = io.WriteString(w, `{"items":[{"userId":"u1","email":"a@b.com","tier":"Pro"}],"continuationToken":"TOKEN1"}`)
	}))
	defer server.Close()

	env := Env{In: strings.NewReader("n\n"), Out: io.Discard, Err: io.Discard}
	code := runListUsers(context.Background(), clientFor(server), env, ParseArgs([]string{"list-users"}))
	if code != exitOK {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1 (declined next page)", requests)
	}
}

// TestRunListUsers_FetchesSummaryOncePerRun asserts the first-page summary
// header is fetched from /v1/admin/stats exactly once across a two-page list —
// not re-fetched on page 2 — and is printed above the table.
func TestRunListUsers_FetchesSummaryOncePerRun(t *testing.T) {
	t.Parallel()
	var statsHits, userHits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/admin/stats" {
			statsHits++
			_, _ = io.WriteString(w, testStatsJSON)
			return
		}
		userHits++
		if r.URL.Query().Get("continuationToken") == "" {
			_, _ = io.WriteString(w, `{"items":[{"userId":"u1","email":"a@b.com","tier":"Pro"}],"continuationToken":"TOKEN1"}`)
			return
		}
		_, _ = io.WriteString(w, `{"items":[{"userId":"u2","email":"c@d.com","tier":"Free"}],"continuationToken":null}`)
	}))
	defer server.Close()

	var out strings.Builder
	env := Env{In: strings.NewReader("y\n"), Out: &out, Err: io.Discard}
	code := runListUsers(context.Background(), clientFor(server), env, ParseArgs([]string{"list-users", "--page-size", "1"}))
	if code != exitOK {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if statsHits != 1 {
		t.Fatalf("stats endpoint hit %d times, want exactly 1 (first page only)", statsHits)
	}
	if userHits != 2 {
		t.Fatalf("users endpoint hit %d times, want 2 (both pages)", userHits)
	}
	if got := out.String(); !strings.Contains(got, "2 users") || !strings.Contains(got, "paying 1") {
		t.Fatalf("stdout missing first-page summary header:\n%s", got)
	}
}

// TestRunListUsers_SummaryFetchFailureDegradesGracefully asserts a failing stats
// fetch does not abort the command: the users table still renders and exit is 0.
func TestRunListUsers_SummaryFetchFailureDegradesGracefully(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/admin/stats" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, "stats boom")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"items":[{"userId":"u1","email":"a@b.com","tier":"Pro"}],"continuationToken":null}`)
	}))
	defer server.Close()

	var out strings.Builder
	env := Env{In: strings.NewReader(""), Out: &out, Err: io.Discard}
	code := runListUsers(context.Background(), clientFor(server), env, ParseArgs([]string{"list-users"}))
	if code != exitOK {
		t.Fatalf("exit code = %d, want 0 (stats-header failure must not abort)", code)
	}
	got := out.String()
	if !strings.Contains(got, "u1") || !strings.Contains(got, "a@b.com") {
		t.Fatalf("users table must still render when the summary fetch fails:\n%s", got)
	}
	// The convenience header is skipped silently — no summary line.
	if strings.Contains(got, "paying") {
		t.Fatalf("summary line must be absent when stats fetch failed:\n%s", got)
	}
}

func TestRunListUsers_InvalidPageSizeReturns1(t *testing.T) {
	t.Parallel()
	env, _, errBuf := captureEnv()
	code := runListUsers(context.Background(), dummyClient(), env, ParseArgs([]string{"list-users", "--page-size", "0"}))
	if code != exitUsage {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "Invalid --page-size: must be a positive integer") {
		t.Fatalf("stderr = %q, want invalid page-size message", errBuf.String())
	}
}

func TestRunListUsers_APIErrorReturns2(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "boom")
	}))
	defer server.Close()

	env, _, errBuf := captureEnv()
	code := runListUsers(context.Background(), clientFor(server), env, ParseArgs([]string{"list-users"}))
	if code != exitRuntime {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "API error (500): boom") {
		t.Fatalf("stderr = %q, want API error (500)", errBuf.String())
	}
}
