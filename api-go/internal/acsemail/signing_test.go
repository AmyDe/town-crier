package acsemail

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
	"testing"
)

func TestParseConnectionString_ExtractsEndpointAndKey(t *testing.T) {
	t.Parallel()

	const cs = "endpoint=https://my-acs.communication.azure.com/;accesskey=c2VjcmV0LWtleS1iYXNlNjQ="
	creds, err := parseConnectionString(cs)
	if err != nil {
		t.Fatalf("parseConnectionString: %v", err)
	}
	if creds.endpoint != "https://my-acs.communication.azure.com" {
		t.Errorf("endpoint = %q, want trailing slash trimmed", creds.endpoint)
	}
	if creds.accessKey.Expose() != "c2VjcmV0LWtleS1iYXNlNjQ=" {
		t.Errorf("accessKey = %q, want the base64 key", creds.accessKey.Expose())
	}
}

func TestParseConnectionString_CaseInsensitiveKeys(t *testing.T) {
	t.Parallel()

	const cs = "Endpoint=https://acs.example.com/;AccessKey=YWJjZA=="
	creds, err := parseConnectionString(cs)
	if err != nil {
		t.Fatalf("parseConnectionString: %v", err)
	}
	if creds.endpoint != "https://acs.example.com" {
		t.Errorf("endpoint = %q", creds.endpoint)
	}
	if creds.accessKey.Expose() != "YWJjZA==" {
		t.Errorf("accessKey = %q", creds.accessKey.Expose())
	}
}

func TestParseConnectionString_RejectsMalformed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cs   string
	}{
		{"empty", ""},
		{"no accesskey", "endpoint=https://acs.example.com/"},
		{"no endpoint", "accesskey=YWJjZA=="},
		{"garbage", "this is not a connection string"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := parseConnectionString(tc.cs); err == nil {
				t.Fatalf("expected error for %q, got nil", tc.cs)
			}
		})
	}
}

// TestSignRequest_MatchesKnownVector pins the ACS HMAC-SHA256 contract against
// an independently-computed signature. The string-to-sign is
// "VERB\n{path+query}\n{date};{host};{contentHash}" signed with the
// base64-decoded access key; this recomputes it the same way and asserts the
// signer produced the identical header.
func TestSignRequest_MatchesKnownVector(t *testing.T) {
	t.Parallel()

	const (
		accessKeyB64 = "dGVzdC1zaWduaW5nLWtleQ==" // "test-signing-key"
		host         = "my-acs.communication.azure.com"
		method       = http.MethodPost
		pathAndQuery = "/emails:send?api-version=2023-03-31"
		date         = "Mon, 01 Jan 2024 00:00:00 GMT"
	)
	body := []byte(`{"hello":"world"}`)

	creds := credentials{
		endpoint:  "https://" + host,
		accessKey: newSecret(accessKeyB64),
	}

	contentHash := computeContentHash(body)
	gotAuth, err := signRequest(creds, method, pathAndQuery, host, date, contentHash)
	if err != nil {
		t.Fatalf("signRequest: %v", err)
	}

	// Recompute the expected signature independently.
	stringToSign := method + "\n" + pathAndQuery + "\n" + date + ";" + host + ";" + contentHash
	rawKey, err := base64.StdEncoding.DecodeString(accessKeyB64)
	if err != nil {
		t.Fatalf("decode key: %v", err)
	}
	mac := hmac.New(sha256.New, rawKey)
	mac.Write([]byte(stringToSign))
	wantSig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	wantAuth := "HMAC-SHA256 SignedHeaders=x-ms-date;host;x-ms-content-sha256&Signature=" + wantSig

	if gotAuth != wantAuth {
		t.Errorf("authorization header mismatch\n got: %q\nwant: %q", gotAuth, wantAuth)
	}
}

func TestComputeContentHash_IsBase64SHA256(t *testing.T) {
	t.Parallel()

	body := []byte("the quick brown fox")
	got := computeContentHash(body)

	sum := sha256.Sum256(body)
	want := base64.StdEncoding.EncodeToString(sum[:])
	if got != want {
		t.Errorf("contentHash = %q, want %q", got, want)
	}
}

func TestSignRequest_RejectsNonBase64Key(t *testing.T) {
	t.Parallel()

	creds := credentials{
		endpoint:  "https://acs.example.com",
		accessKey: newSecret("!!! not base64 !!!"),
	}
	_, err := signRequest(creds, http.MethodPost, "/emails:send", "acs.example.com", "date", "hash")
	if err == nil {
		t.Fatal("expected error for a non-base64 access key, got nil")
	}
}

func TestSignedHeaders_AreLowercaseAndOrdered(t *testing.T) {
	t.Parallel()

	creds := credentials{
		endpoint:  "https://acs.example.com",
		accessKey: newSecret("YWJjZA=="),
	}
	auth, err := signRequest(creds, http.MethodPost, "/p", "acs.example.com", "d", "h")
	if err != nil {
		t.Fatalf("signRequest: %v", err)
	}
	if !strings.Contains(auth, "SignedHeaders=x-ms-date;host;x-ms-content-sha256") {
		t.Errorf("auth header lacks the canonical SignedHeaders list: %q", auth)
	}
}
