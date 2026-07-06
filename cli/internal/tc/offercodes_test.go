package tc

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func dummyClient() *Client {
	return NewClient(Config{URL: "http://127.0.0.1:1", APIKey: "sk-test"})
}

func captureEnv() (Env, *bytes.Buffer, *bytes.Buffer) {
	var out, errBuf bytes.Buffer
	return Env{In: strings.NewReader(""), Out: &out, Err: &errBuf}, &out, &errBuf
}

func TestRunGenerateOfferCodes_ValidationFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		args        []string
		wantErrPart string
	}{
		{"count missing", []string{"generate-offer-codes", "--tier", "Pro", "--duration-days", "30", "--label", "l"}, "Missing required argument: --count"},
		{"tier missing", []string{"generate-offer-codes", "--count", "10", "--duration-days", "30", "--label", "l"}, "Missing required argument: --tier"},
		{"duration missing", []string{"generate-offer-codes", "--count", "10", "--tier", "Pro", "--label", "l"}, "Missing required argument: --duration-days"},
		{"label missing", []string{"generate-offer-codes", "--count", "10", "--tier", "Pro", "--duration-days", "30"}, "Missing required argument: --label"},
		{"count not integer", []string{"generate-offer-codes", "--count", "abc", "--tier", "Pro", "--duration-days", "30", "--label", "l"}, "Invalid --count: must be an integer between 1 and 1000"},
		{"count below range", []string{"generate-offer-codes", "--count", "0", "--tier", "Pro", "--duration-days", "30", "--label", "l"}, "Invalid --count"},
		{"count above range", []string{"generate-offer-codes", "--count", "1001", "--tier", "Pro", "--duration-days", "30", "--label", "l"}, "Invalid --count"},
		{"tier invalid free", []string{"generate-offer-codes", "--count", "10", "--tier", "Free", "--duration-days", "30", "--label", "l"}, "Invalid tier: Free. Must be one of: Personal, Pro"},
		{"tier unknown", []string{"generate-offer-codes", "--count", "10", "--tier", "Enterprise", "--duration-days", "30", "--label", "l"}, "Invalid tier: Enterprise"},
		{"duration not integer", []string{"generate-offer-codes", "--count", "10", "--tier", "Pro", "--duration-days", "abc", "--label", "l"}, "Invalid --duration-days: must be an integer between 1 and 365"},
		{"duration below range", []string{"generate-offer-codes", "--count", "10", "--tier", "Pro", "--duration-days", "0", "--label", "l"}, "Invalid --duration-days"},
		{"duration above range", []string{"generate-offer-codes", "--count", "10", "--tier", "Pro", "--duration-days", "366", "--label", "l"}, "Invalid --duration-days"},
		{"label blank", []string{"generate-offer-codes", "--count", "10", "--tier", "Pro", "--duration-days", "30", "--label", "   "}, "Invalid --label: must not be blank"},
		{"max-redemptions not integer", []string{"generate-offer-codes", "--count", "10", "--tier", "Pro", "--duration-days", "30", "--label", "l", "--max-redemptions", "abc"}, "Invalid --max-redemptions: must be an integer between 1 and 10000"},
		{"max-redemptions below range", []string{"generate-offer-codes", "--count", "10", "--tier", "Pro", "--duration-days", "30", "--label", "l", "--max-redemptions", "0"}, "Invalid --max-redemptions"},
		{"max-redemptions above range", []string{"generate-offer-codes", "--count", "10", "--tier", "Pro", "--duration-days", "30", "--label", "l", "--max-redemptions", "10001"}, "Invalid --max-redemptions"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			env, _, errBuf := captureEnv()
			code := runGenerateOfferCodes(context.Background(), dummyClient(), env, ParseArgs(tc.args))
			if code != exitUsage {
				t.Fatalf("exit code = %d, want %d", code, exitUsage)
			}
			if !strings.Contains(errBuf.String(), tc.wantErrPart) {
				t.Fatalf("stderr = %q, want to contain %q", errBuf.String(), tc.wantErrPart)
			}
		})
	}
}

func TestRunGenerateOfferCodes_NormalisesTierCasing(t *testing.T) {
	t.Parallel()
	// A lowercase tier passes validation; we can't reach the network here, but the
	// command should not bail with a validation error before attempting the POST.
	env := Env{In: strings.NewReader(""), Out: io.Discard, Err: io.Discard}
	code := runGenerateOfferCodes(context.Background(), dummyClient(), env, ParseArgs([]string{
		"generate-offer-codes", "--count", "5", "--tier", "pro", "--duration-days", "30", "--label", "campaign",
	}))
	// 127.0.0.1:1 refuses the connection, so a valid request fails at the network
	// stage with exit code 2 — proving validation (including casing) passed.
	if code != exitRuntime {
		t.Fatalf("exit code = %d, want %d (network failure after passing validation)", code, exitRuntime)
	}
}

func TestRunListOfferCodes_RendersTable(t *testing.T) {
	t.Parallel()
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[`+
			`{"code":"ABCD-EFGH-JKMN","label":"creator-campaign","tier":"Pro","durationDays":30,`+
			`"maxRedemptions":3,"redemptionCount":2,"createdAt":"2026-06-01T09:00:00Z","lastRedeemedAt":"2026-06-15T10:00:00Z"},`+
			`{"code":"NPQR-STVW-XYZ0","label":"unused","tier":"Personal","durationDays":7,`+
			`"maxRedemptions":1,"redemptionCount":0,"createdAt":"2026-06-02T09:00:00Z","lastRedeemedAt":null}`+
			`]`)
	}))
	defer server.Close()

	env, out, _ := captureEnv()
	code := runListOfferCodes(context.Background(), clientFor(server), env, ParseArgs([]string{"list-offer-codes"}))
	if code != exitOK {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if gotPath != "/v1/admin/offer-codes" {
		t.Fatalf("path = %q, want /v1/admin/offer-codes (no ?label given)", gotPath)
	}

	got := out.String()
	for _, want := range []string{
		"Code", "Label", "Tier", "Duration", "Redeemed", "Created", "LastRedeemed",
		"ABCD-EFGH-JKMN", "creator-campaign", "Pro", "30d", "2/3", "2026-06-01", "2026-06-15",
		"NPQR-STVW-XYZ0", "unused", "Personal", "7d", "0/1", "2026-06-02", "-",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q; got:\n%s", want, got)
		}
	}
}

func TestRunListOfferCodes_PassesLabelFilter(t *testing.T) {
	t.Parallel()
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	}))
	defer server.Close()

	env, out, _ := captureEnv()
	code := runListOfferCodes(context.Background(), clientFor(server), env, ParseArgs([]string{"list-offer-codes", "--label", "creator campaign"}))
	if code != exitOK {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if gotPath != "/v1/admin/offer-codes?label=creator+campaign" {
		t.Fatalf("path = %q, want label query-escaped", gotPath)
	}
	// An empty result still renders the header row.
	if got := out.String(); !strings.Contains(got, "Code") || !strings.Contains(got, "Label") {
		t.Fatalf("empty listing must still render a header:\n%s", got)
	}
}

func TestRunListOfferCodes_APIErrorReturns2(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "boom")
	}))
	defer server.Close()

	env, _, errBuf := captureEnv()
	code := runListOfferCodes(context.Background(), clientFor(server), env, ParseArgs([]string{"list-offer-codes"}))
	if code != exitRuntime {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "API error (500): boom") {
		t.Fatalf("stderr = %q, want API error (500)", errBuf.String())
	}
}
