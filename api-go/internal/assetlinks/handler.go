// Package assetlinks serves the Android Digital Asset Links document on the
// public share host (GH#782, Android App Links). Android's package manager
// fetches GET /.well-known/assetlinks.json over HTTPS to verify a package
// claiming an https intent-filter with android:autoVerify="true" — the
// Android equivalent of internal/aasa's apple-app-site-association document,
// served from the same share host.
//
// The endpoint is stateless (no store) and emits only the fixed, public
// association document, so wiring registers it unconditionally alongside
// AASA.
package assetlinks

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// wellKnownPath is the Digital Asset Links location Android's package manager
// fetches. Unlike Apple's AASA path, Google's convention keeps the ".json"
// extension.
const wellKnownPath = "/.well-known/assetlinks.json"

const (
	// relationHandleAllURLs is the only relation Digital Asset Links defines
	// for App Links verification.
	relationHandleAllURLs = "delegate_permission/common.handle_all_urls"
	// namespaceAndroidApp identifies the target as an Android application.
	namespaceAndroidApp = "android_app"
)

// Package pairs an Android application id with the SHA-256 certificate
// fingerprints (colon-separated uppercase hex, the format Digital Asset
// Links requires) of the signing key(s) Play Console reports for it — the
// upload key and/or the Play App Signing key.
type Package struct {
	Name         string
	Fingerprints []string
}

// statement, target model the Digital Asset Links JSON so it is marshalled
// from typed data (always valid) rather than string-built, mirroring
// internal/aasa's document/applinks/detail/componentEntry types.
type statement struct {
	Relation []string `json:"relation"`
	Target   target   `json:"target"`
}

type target struct {
	Namespace              string   `json:"namespace"`
	PackageName            string   `json:"package_name"`
	SHA256CertFingerprints []string `json:"sha256_cert_fingerprints"`
}

type handler struct {
	body   []byte
	logger *slog.Logger
}

// Routes registers the anonymous assetlinks endpoint for the given packages.
// A Package with no fingerprints yet (e.g. uk.towncrierapp.mobile, pending
// Play Console enrolment, #779) is skipped rather than published with an
// empty or placeholder fingerprint that would fail verification anyway.
//
// The document is marshalled once at wiring time so the handler does no
// per-request work, mirroring internal/aasa's Routes.
func Routes(mux *http.ServeMux, packages []Package, logger *slog.Logger) {
	statements := make([]statement, 0, len(packages))
	for _, p := range packages {
		if len(p.Fingerprints) == 0 {
			continue
		}
		statements = append(statements, statement{
			Relation: []string{relationHandleAllURLs},
			Target: target{
				Namespace:              namespaceAndroidApp,
				PackageName:            p.Name,
				SHA256CertFingerprints: p.Fingerprints,
			},
		})
	}

	body, err := json.Marshal(statements)
	if err != nil {
		logger.Error("assetlinks: marshal document", "error", err)
		return
	}
	h := &handler{body: body, logger: logger}
	mux.HandleFunc("GET "+wellKnownPath, h.serve)
}

func (h *handler) serve(w http.ResponseWriter, r *http.Request) {
	// Google's verifier requires exactly application/json.
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(h.body); err != nil {
		h.logger.ErrorContext(r.Context(), "write assetlinks response", "error", err)
	}
}
