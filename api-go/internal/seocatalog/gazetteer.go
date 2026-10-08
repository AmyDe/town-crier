// Package seocatalog maintains the Postgres catalog behind the programmatic SEO
// pages: the embedded town gazetteer, the stored application-to-town
// assignment, and the daily-recomputed page set (seo_pages, seo_redirects).
package seocatalog

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

//go:embed resources/towns.json
var embeddedTowns []byte

// Town is one gazetteer record. Every town uses the same
// AssignmentRadiusMetres radius.
type Town struct {
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
	AuthorityID int     `json:"authorityId"`
	Population  int     `json:"population"`
}

// Gazetteer is a parsed town list plus the SHA-256 (hex) of its source bytes,
// used to detect a changed gazetteer between runs.
type Gazetteer struct {
	Towns []Town
	Hash  string
}

// ParseGazetteer decodes a gazetteer JSON array.
func ParseGazetteer(raw []byte) (Gazetteer, error) {
	var towns []Town
	if err := json.Unmarshal(raw, &towns); err != nil {
		return Gazetteer{}, fmt.Errorf("parse town gazetteer: %w", err)
	}
	sum := sha256.Sum256(raw)
	return Gazetteer{Towns: towns, Hash: hex.EncodeToString(sum[:])}, nil
}

// EmbeddedGazetteer returns the gazetteer compiled into the binary.
func EmbeddedGazetteer() (Gazetteer, error) {
	return ParseGazetteer(embeddedTowns)
}
