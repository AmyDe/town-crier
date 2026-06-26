package main

import (
	"net/url"
	"strings"
	"testing"
)

// adminUPN is a realistic Entra external-guest admin UPN: it contains the two
// characters (# and @) that make a token-as-password DSN a percent-encoding
// trap if naively concatenated.
const adminUPN = "amy_salter.uk#EXT#@amysalter.onmicrosoft.com"

// fakeToken stands in for an Entra access token. Real tokens are base64url JWTs
// (only unreserved chars), but the password must still survive characters that
// require escaping in userinfo, so the fixture deliberately includes '/', ':'
// and '@'.
const fakeToken = "header.payload.sig/na:ture@v1"

func TestMigrateDSN(t *testing.T) {
	t.Parallel()

	const host = "psql-town-crier-shared.postgres.database.azure.com"
	const db = "town_crier_dev"

	tests := []struct {
		name        string
		sslMode     string
		wantSSLMode string
	}{
		{name: "explicit sslmode is preserved", sslMode: "verify-full", wantSSLMode: "verify-full"},
		{name: "empty sslmode defaults to require", sslMode: "", wantSSLMode: "require"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dsn := migrateDSN(host, db, adminUPN, tc.sslMode, fakeToken)

			// Round-trip through url.Parse: if the UPN '#'/'@' and the token's
			// reserved chars are correctly percent-encoded, parsing decodes them
			// back to the exact inputs. A naive concat would mis-split here.
			u, err := url.Parse(dsn)
			if err != nil {
				t.Fatalf("migrateDSN produced an unparseable DSN %q: %v", dsn, err)
			}
			if u.Scheme != "postgres" {
				t.Errorf("scheme = %q, want %q", u.Scheme, "postgres")
			}
			if u.Host != host {
				t.Errorf("host = %q, want %q", u.Host, host)
			}
			if u.Path != "/"+db {
				t.Errorf("path = %q, want %q", u.Path, "/"+db)
			}
			if got := u.User.Username(); got != adminUPN {
				t.Errorf("username = %q, want %q", got, adminUPN)
			}
			pw, ok := u.User.Password()
			if !ok {
				t.Fatal("DSN carries no password (token)")
			}
			if pw != fakeToken {
				t.Errorf("password = %q, want %q", pw, fakeToken)
			}
			if got := u.Query().Get("sslmode"); got != tc.wantSSLMode {
				t.Errorf("sslmode = %q, want %q", got, tc.wantSSLMode)
			}
		})
	}
}

// TestMigrateDSN_PercentEncodesUPN asserts the raw DSN string actually carries
// the encoded forms, so the encoding intent is explicit and not silently lost
// to a future refactor that round-trips differently.
func TestMigrateDSN_PercentEncodesUPN(t *testing.T) {
	t.Parallel()

	dsn := migrateDSN("host.example.com", "town_crier_dev", adminUPN, "require", fakeToken)

	if strings.Contains(dsn, "#") {
		t.Errorf("DSN contains a raw '#'; the UPN '#' must be percent-encoded: %q", dsn)
	}
	if !strings.Contains(dsn, "%23") {
		t.Errorf("DSN is missing the encoded '#' (%%23): %q", dsn)
	}
	if !strings.Contains(dsn, "%40") {
		t.Errorf("DSN is missing the encoded '@' (%%40) from the UPN: %q", dsn)
	}
	// Exactly one literal '@' may remain: the userinfo/host separator. Both the
	// UPN's '@' and the token's '@' must have been encoded to %40.
	if n := strings.Count(dsn, "@"); n != 1 {
		t.Errorf("DSN has %d literal '@', want exactly 1 (userinfo separator): %q", n, dsn)
	}
}
