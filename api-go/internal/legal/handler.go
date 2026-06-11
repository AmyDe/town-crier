// Package legal serves GET /v1/legal/{documentType} — the Privacy Policy and
// Terms of Service. The documents are embedded at build time and served as
// compact camelCase JSON, byte-for-byte matching the .NET API's Results.Ok
// output (which re-serializes the same source files compactly).
//
// The embedded resources/{privacy,terms}.json are byte-identical copies of the
// canonical files at api/src/town-crier.application/Legal/Resources. Go cannot
// embed files outside its own module, so the copies live here and are guarded
// against drift by scripts/check-legal-drift.sh.
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

// document mirrors the .NET GetLegalDocumentResult record. Field order matches
// the record declaration so the compact JSON wire order is identical to .NET's
// source-generated output.
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
// by the upper-cased type to match .NET's ToUpperInvariant lookup.
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

// loadCompact reads an embedded document and re-encodes it compactly in the
// .NET field order. Round-tripping through the struct (rather than serving the
// raw pretty-printed file) is what reproduces .NET's compact wire bytes.
func loadCompact(file string) ([]byte, error) {
	raw, err := resources.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("read embedded %s: %w", file, err)
	}
	var doc document
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", file, err)
	}
	// Disable HTML escaping: .NET's System.Text.Json does not escape <, >, or &
	// in this code path, so escaping them (Go's default) would break byte
	// parity the moment legal copy includes one of those characters.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("marshal %s: %w", file, err)
	}
	// Encoder.Encode appends a trailing newline; trim it for compact parity.
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// lookup returns the pre-encoded document for the given route value, matching
// .NET's case-insensitive ToUpperInvariant switch. The bool reports whether a
// known type was found.
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
		// Parity: .NET returns Results.NotFound() (bodyless); the PascalCase
		// envelope is backfilled by middleware.ErrorBody, as in .NET.
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := w.Write(doc); err != nil {
		h.logger.ErrorContext(r.Context(), "write legal response", "documentType", r.PathValue("documentType"), "error", err)
	}
}
