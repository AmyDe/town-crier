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
		"generate-offer-codes", "--count", "2", "--tier", "Pro", "--duration-days", "30",
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
	if gotBody != (generateOfferCodesRequest{Count: 2, Tier: "Pro", DurationDays: 30}) {
		t.Fatalf("request body = %+v, want {2 Pro 30}", gotBody)
	}
	if got := out.String(); got != "AAAA-BBBB\nCCCC-DDDD\n" {
		t.Fatalf("stdout = %q, want the two codes", got)
	}
	if !strings.Contains(errBuf.String(), "Generated 2 codes: Pro tier, 30 days duration") {
		t.Fatalf("stderr = %q, want summary line", errBuf.String())
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
		"generate-offer-codes", "--count", "2", "--tier", "Pro", "--duration-days", "30",
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"items":[{"userId":"u1","email":"a@b.com","tier":"Pro"},{"userId":"u2","email":null,"tier":"Free"}],"continuationToken":null}`)
	}))
	defer server.Close()

	env, out, _ := captureEnv()
	code := runListUsers(context.Background(), clientFor(server), env, ParseArgs([]string{"list-users"}))
	if code != exitOK {
		t.Fatalf("exit code = %d, want 0", code)
	}

	got := out.String()
	for _, want := range []string{"UserId", "Email", "Tier", strings.Repeat("-", 66), "u1", "a@b.com", "u2", "(none)", "Free"} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q; got:\n%s", want, got)
		}
	}
}

func TestRunListUsers_PaginatesOnYes(t *testing.T) {
	t.Parallel()
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.String())
		w.Header().Set("Content-Type", "application/json")
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
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
