package assetlinks

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// assetLinksStatement mirrors the on-the-wire Digital Asset Links shape so the
// test parses the served body back and asserts the exact contract (relation,
// namespace, package name, fingerprints) rather than matching a byte string.
type assetLinksStatement struct {
	Relation []string `json:"relation"`
	Target   struct {
		Namespace              string   `json:"namespace"`
		PackageName            string   `json:"package_name"`
		SHA256CertFingerprints []string `json:"sha256_cert_fingerprints"`
	} `json:"target"`
}

func TestAssetLinks_ServesJSONDocument(t *testing.T) {
	t.Parallel()

	const pkgName = "uk.towncrierapp.mobile.dev"
	const fingerprint = "75:2F:87:AF:52:B6:4D:33:71:ED:77:2A:2A:1C:D9:7A:A4:67:9E:1A:17:F0:9F:FD:92:12:D6:55:92:FD:0E:07"

	mux := http.NewServeMux()
	Routes(mux, []Package{{Name: pkgName, Fingerprints: []string{fingerprint}}}, slog.New(slog.DiscardHandler))

	// Android's package manager fetches the extensionless well-known path over
	// HTTPS to verify the autoVerify intent filter.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/.well-known/assetlinks.json", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	var statements []assetLinksStatement
	if err := json.Unmarshal(rec.Body.Bytes(), &statements); err != nil {
		t.Fatalf("body is not valid JSON: %v (%q)", err, rec.Body.String())
	}
	if len(statements) != 1 {
		t.Fatalf("statements = %d, want 1", len(statements))
	}
	s := statements[0]
	if len(s.Relation) != 1 || s.Relation[0] != "delegate_permission/common.handle_all_urls" {
		t.Errorf("relation = %v, want [delegate_permission/common.handle_all_urls]", s.Relation)
	}
	if s.Target.Namespace != "android_app" {
		t.Errorf("target.namespace = %q, want android_app", s.Target.Namespace)
	}
	if s.Target.PackageName != pkgName {
		t.Errorf("target.package_name = %q, want %q", s.Target.PackageName, pkgName)
	}
	if len(s.Target.SHA256CertFingerprints) != 1 || s.Target.SHA256CertFingerprints[0] != fingerprint {
		t.Errorf("target.sha256_cert_fingerprints = %v, want [%q]", s.Target.SHA256CertFingerprints, fingerprint)
	}
}
