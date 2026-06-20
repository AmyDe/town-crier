package acsemail

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// ErrInvalidConnectionString reports that the ACS connection string is missing
// the endpoint or accesskey component, or is otherwise unparseable.
var ErrInvalidConnectionString = errors.New("acsemail: invalid connection string")

// credentials holds the ACS account endpoint and base64 access key parsed from
// the connection string.
type credentials struct {
	endpoint  string
	accessKey platform.SecretString
}

// newSecret wraps a raw value in the redacting SecretString, re-exported locally
// so signing tests don't reach into the platform package.
func newSecret(v string) platform.SecretString { return platform.NewSecret(v) }

// parseConnectionString splits an ACS connection string of the form
// "endpoint=https://...;accesskey=<base64>" into its endpoint (trailing slash
// trimmed) and base64 access key. Keys are matched case-insensitively, matching
// the "endpoint=https://...;accesskey=<base64>" format ACS expects.
func parseConnectionString(cs string) (credentials, error) {
	var endpoint, key string
	for _, part := range strings.Split(cs, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "endpoint":
			endpoint = strings.TrimRight(strings.TrimSpace(value), "/")
		case "accesskey":
			key = strings.TrimSpace(value)
		}
	}

	if endpoint == "" {
		return credentials{}, fmt.Errorf("%w: missing endpoint", ErrInvalidConnectionString)
	}
	if key == "" {
		return credentials{}, fmt.Errorf("%w: missing accesskey", ErrInvalidConnectionString)
	}
	return credentials{endpoint: endpoint, accessKey: newSecret(key)}, nil
}

// computeContentHash returns the base64-encoded SHA-256 of the request body, the
// value sent in the x-ms-content-sha256 header and folded into the signature.
func computeContentHash(body []byte) string {
	sum := sha256.Sum256(body)
	return base64.StdEncoding.EncodeToString(sum[:])
}

// signRequest builds the ACS HMAC-SHA256 Authorization header. The string to
// sign is "VERB\n{pathAndQuery}\n{date};{host};{contentHash}", signed with the
// base64-decoded access key; the result is wrapped in the
// "HMAC-SHA256 SignedHeaders=...&Signature=..." envelope ACS expects.
func signRequest(creds credentials, method, pathAndQuery, host, date, contentHash string) (string, error) {
	rawKey, err := base64.StdEncoding.DecodeString(creds.accessKey.Expose())
	if err != nil {
		return "", fmt.Errorf("acsemail: decode access key: %w", err)
	}

	stringToSign := method + "\n" + pathAndQuery + "\n" + date + ";" + host + ";" + contentHash

	mac := hmac.New(sha256.New, rawKey)
	mac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return "HMAC-SHA256 SignedHeaders=x-ms-date;host;x-ms-content-sha256&Signature=" + signature, nil
}
