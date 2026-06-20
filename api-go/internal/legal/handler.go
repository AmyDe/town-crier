// Package legal serves GET /v1/legal/{documentType} — the Privacy Policy and
// Terms of Service. The documents are embedded at build time and served as
// compact camelCase JSON.
//
// The source files are api-go/internal/legal/resources/{privacy,terms}.json.
// The iOS app bundles a byte-equal mirror; scripts/check-legal-drift.sh
// validates parity on every PR.
package legal

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

//go:embed resources/privacy.json resources/terms.json
var resources embed.FS

// document is the JSON shape for a legal document response. Field order is
// significant: the compact JSON wire order follows declaration order.
type document struct {
	DocumentType string    `json:"documentType"`
	Title        string    `json:"title"`
	LastUpdated  string    `json:"lastUpdated"`
	Sections     []section `json:"sections"`
}

type section struct {
	Heading string `json:"heading"`
	Body    string `json:"body"`
}

// store holds the pre-encoded compact JSON for each known document type, keyed
// by the upper-cased type so the lookup is case-insensitive.
type store struct {
	docs map[string][]byte
}

// newStore loads and compacts the embedded documents once. It panics on a
// malformed embedded resource because that is a build-time invariant, not a
// runtime condition — the only acceptable panic site per the standards.
func newStore() *store {
	s := &store{docs: make(map[string][]byte, 2)}
	for key, file := range map[string]string{
		"PRIVACY": "resources/privacy.json",
		"TERMS":   "resources/terms.json",
	} {
		encoded, err := loadCompact(file)
		if err != nil {
			panic(fmt.Sprintf("legal: load %s: %v", file, err))
		}
		s.docs[key] = encoded
	}
	return s
}

// loadCompact reads an embedded document and re-encodes it compactly in
// struct field order. Round-tripping through the struct (rather than serving
// the raw pretty-printed file) produces compact wire bytes in declaration order.
func loadCompact(file string) ([]byte, error) {
	raw, err := resources.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("read embedded %s: %w", file, err)
	}
	var doc document
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", file, err)
	}
	// Disable HTML escaping: JSON-encoding legal content must not escape <, >,
	// or & (Go's json.Encoder escapes them by default, which would corrupt the
	// text).
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("marshal %s: %w", file, err)
	}
	// Encoder.Encode appends a trailing newline; trim it for compact parity.
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// lookup returns the pre-encoded document for the given route value
// (case-insensitive). The bool reports whether a known type was found.
func (s *store) lookup(documentType string) ([]byte, bool) {
	doc, ok := s.docs[strings.ToUpper(documentType)]
	return doc, ok
}

// Routes registers the legal endpoint on mux.
func Routes(mux *http.ServeMux, logger *slog.Logger) {
	h := handler{store: newStore(), logger: logger}
	mux.HandleFunc("GET /v1/legal/{documentType}", h.get)
}

type handler struct {
	store  *store
	logger *slog.Logger
}

func (h handler) get(w http.ResponseWriter, r *http.Request) {
	doc, ok := h.store.lookup(r.PathValue("documentType"))
	if !ok {
		// Returns a bodyless 404; the PascalCase envelope is backfilled by
		// middleware.ErrorBody.
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := w.Write(doc); err != nil {
		h.logger.ErrorContext(r.Context(), "write legal response", "documentType", r.PathValue("documentType"), "error", err)
	}
}
