// Package seocatalog maintains the Postgres catalog behind the programmatic SEO
// pages: the embedded town gazetteer and the daily-recomputed page set
// (seo_pages, seo_redirects, seo_town_applications).
package seocatalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed resources/towns.json
var embeddedTowns []byte

// Town is one gazetteer record. Every town uses the same TownRadiusMetres
// radius.
type Town struct {
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
	AuthorityID int     `json:"authorityId"`
	Population  int     `json:"population"`
}

// Gazetteer is a parsed town list.
type Gazetteer struct {
	Towns []Town
}

// ParseGazetteer decodes a gazetteer JSON array.
func ParseGazetteer(raw []byte) (Gazetteer, error) {
	var towns []Town
	if err := json.Unmarshal(raw, &towns); err != nil {
		return Gazetteer{}, fmt.Errorf("parse town gazetteer: %w", err)
	}
	return Gazetteer{Towns: towns}, nil
}

// EmbeddedGazetteer returns the gazetteer compiled into the binary.
func EmbeddedGazetteer() (Gazetteer, error) {
	return ParseGazetteer(embeddedTowns)
}
