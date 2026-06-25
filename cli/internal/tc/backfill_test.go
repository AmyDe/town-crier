package tc

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunBackfillWatchZoneLocation_SuccessReturns0(t *testing.T) {
	t.Parallel()
	var gotKey, gotPath, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get(apiKeyHeader)
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"total":10,"backfilled":3,"alreadyHad":7}`)
	}))
	defer server.Close()

	env, out, errBuf := captureEnv()
	code := runBackfillWatchZoneLocation(context.Background(), clientFor(server), env, ParseArgs([]string{
		"backfill-watchzone-location",
	}))

	if code != exitOK {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, errBuf.String())
	}
	if gotMethod != http.MethodPost || gotPath != "/v1/admin/watchzones/backfill-location" {
		t.Fatalf("request = %s %s, want POST /v1/admin/watchzones/backfill-location", gotMethod, gotPath)
	}
	if gotKey != "sk-test" {
		t.Fatalf("X-Admin-Key = %q, want sk-test", gotKey)
	}
	if got := out.String(); !strings.Contains(got, "Backfilled location on 3 of 10 watch zones (7 already had it)") {
		t.Fatalf("stdout = %q, want reconciled summary", got)
	}
}

func TestRunBackfillWatchZoneLocation_APIErrorReturns2(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "boom")
	}))
	defer server.Close()

	env, _, errBuf := captureEnv()
	code := runBackfillWatchZoneLocation(context.Background(), clientFor(server), env, ParseArgs([]string{
		"backfill-watchzone-location",
	}))

	if code != exitRuntime {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "API error (500): boom") {
		t.Fatalf("stderr = %q, want API error (500)", errBuf.String())
	}
}

func TestRunBackfillWatchZoneBoundingBox_SuccessReturns0(t *testing.T) {
	t.Parallel()
	var gotKey, gotPath, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get(apiKeyHeader)
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"total":10,"backfilled":3,"alreadyHad":7}`)
	}))
	defer server.Close()

	env, out, errBuf := captureEnv()
	code := runBackfillWatchZoneBoundingBox(context.Background(), clientFor(server), env, ParseArgs([]string{
		"backfill-watchzone-bbox",
	}))

	if code != exitOK {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, errBuf.String())
	}
	if gotMethod != http.MethodPost || gotPath != "/v1/admin/watchzones/backfill-bbox" {
		t.Fatalf("request = %s %s, want POST /v1/admin/watchzones/backfill-bbox", gotMethod, gotPath)
	}
	if gotKey != "sk-test" {
		t.Fatalf("X-Admin-Key = %q, want sk-test", gotKey)
	}
	if got := out.String(); !strings.Contains(got, "Backfilled bounding box on 3 of 10 watch zones (7 already had it)") {
		t.Fatalf("stdout = %q, want reconciled summary", got)
	}
}

func TestRunBackfillWatchZoneBoundingBox_APIErrorReturns2(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "boom")
	}))
	defer server.Close()

	env, _, errBuf := captureEnv()
	code := runBackfillWatchZoneBoundingBox(context.Background(), clientFor(server), env, ParseArgs([]string{
		"backfill-watchzone-bbox",
	}))

	if code != exitRuntime {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "API error (500): boom") {
		t.Fatalf("stderr = %q, want API error (500)", errBuf.String())
	}
}
