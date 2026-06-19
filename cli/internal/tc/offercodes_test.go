package tc

import (
	"bytes"
	"context"
	"io"
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
		{"count missing", []string{"generate-offer-codes", "--tier", "Pro", "--duration-days", "30"}, "Missing required argument: --count"},
		{"tier missing", []string{"generate-offer-codes", "--count", "10", "--duration-days", "30"}, "Missing required argument: --tier"},
		{"duration missing", []string{"generate-offer-codes", "--count", "10", "--tier", "Pro"}, "Missing required argument: --duration-days"},
		{"count not integer", []string{"generate-offer-codes", "--count", "abc", "--tier", "Pro", "--duration-days", "30"}, "Invalid --count: must be an integer between 1 and 1000"},
		{"count below range", []string{"generate-offer-codes", "--count", "0", "--tier", "Pro", "--duration-days", "30"}, "Invalid --count"},
		{"count above range", []string{"generate-offer-codes", "--count", "1001", "--tier", "Pro", "--duration-days", "30"}, "Invalid --count"},
		{"tier invalid free", []string{"generate-offer-codes", "--count", "10", "--tier", "Free", "--duration-days", "30"}, "Invalid tier: Free. Must be one of: Personal, Pro"},
		{"tier unknown", []string{"generate-offer-codes", "--count", "10", "--tier", "Enterprise", "--duration-days", "30"}, "Invalid tier: Enterprise"},
		{"duration not integer", []string{"generate-offer-codes", "--count", "10", "--tier", "Pro", "--duration-days", "abc"}, "Invalid --duration-days: must be an integer between 1 and 365"},
		{"duration below range", []string{"generate-offer-codes", "--count", "10", "--tier", "Pro", "--duration-days", "0"}, "Invalid --duration-days"},
		{"duration above range", []string{"generate-offer-codes", "--count", "10", "--tier", "Pro", "--duration-days", "366"}, "Invalid --duration-days"},
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
		"generate-offer-codes", "--count", "5", "--tier", "pro", "--duration-days", "30",
	}))
	// 127.0.0.1:1 refuses the connection, so a valid request fails at the network
	// stage with exit code 2 — proving validation (including casing) passed.
	if code != exitRuntime {
		t.Fatalf("exit code = %d, want %d (network failure after passing validation)", code, exitRuntime)
	}
}
