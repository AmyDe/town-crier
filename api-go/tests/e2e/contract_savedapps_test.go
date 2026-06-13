//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// Valid-token contract scenarios for the iteration-6 saved + planning
// application surface. A save is performed on the .NET side as un-diffed setup
// (both APIs share one Cosmos database), then the read endpoints are diffed.
// The saved-list diff is scoped to the entry this test creates so pre-existing
// rows (and the deferred legacy-migration paths, tc-wans) never affect it.
//
// Quota-safe: the created saved row is always deleted on cleanup; the master
// application record is shared reference data and is left in place.
func TestContract_SavedApplications(t *testing.T) {
	dotnetURL := baseURL(t, "DOTNET_BASE_URL")
	goURL := baseURL(t, "GO_BASE_URL")
	token := integrationToken(t)
	client := &http.Client{Timeout: requestTimeout}

	if setup := watchZoneRequest(t, client, dotnetURL, http.MethodPost, "/v1/me", token, ""); setup.status >= 500 {
		t.Fatalf("setup POST /v1/me on .NET: %d %s", setup.status, setup.body)
	}

	suffix := newZoneSuffix(t)
	name := "CONTRACT-" + suffix  // slash-free; canonical uid = "471/CONTRACT-..."
	canonicalUID := "471/" + name // {areaId}/{name}
	savePath := "/v1/me/saved-applications/" + canonicalUID
	appPath := "/v1/applications/471/" + name
	saveBody := fmt.Sprintf(`{
		"name": %q, "uid": "ABC-%s", "areaName": "City of London", "areaId": 471,
		"address": "1 Contract Street", "postcode": "EC2V 5AE", "description": "Contract test",
		"appType": "Full", "appState": "Permitted", "appSize": "Small",
		"startDate": "2026-01-05", "decidedDate": "2026-03-01", "consultedDate": "2026-01-20",
		"longitude": -0.0931, "latitude": 51.5155,
		"url": "https://planit.example/app", "link": "https://council.example/app",
		"lastDifferent": "2026-03-02T09:30:00+00:00"
	}`, name, suffix)

	// Setup, not a diff: create the master record + saved row on .NET.
	if created := watchZoneRequest(t, client, dotnetURL, http.MethodPut, savePath, token, saveBody); created.status != http.StatusNoContent {
		t.Fatalf("setup save on .NET: status %d body %s", created.status, created.body)
	}
	t.Cleanup(func() {
		_ = watchZoneRequest(t, client, dotnetURL, http.MethodDelete, savePath, token, "")
	})

	// Save is idempotent: re-saving on both APIs is a 204 on the shared state.
	t.Run("PUT save (idempotent)", func(t *testing.T) {
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodPut, savePath, token, saveBody)
	})

	// The created saved entry must be byte-identical between the two APIs.
	t.Run("GET saved-applications entry", func(t *testing.T) {
		want := findSavedEntry(t, watchZoneRequest(t, client, dotnetURL, http.MethodGet, "/v1/me/saved-applications", token, "").body, canonicalUID)
		got := findSavedEntry(t, watchZoneRequest(t, client, goURL, http.MethodGet, "/v1/me/saved-applications", token, "").body, canonicalUID)
		if want == nil || got == nil {
			t.Fatalf("created entry missing: dotnet=%v go=%v", want != nil, got != nil)
		}
		if !jsonEqual(t, got, want) {
			t.Errorf("saved entry: go=%s dotnet=%s", got, want)
		}
	})

	// The master application point read.
	t.Run("GET applications/{authority}/{name}", func(t *testing.T) {
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodGet, appPath, token, "")
	})

	// The watch-zone-derived authority list.
	t.Run("GET application-authorities", func(t *testing.T) {
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodGet, "/v1/me/application-authorities", token, "")
	})

	// A body missing the uid is a 400 with the ApiErrorResponse envelope.
	t.Run("PUT save invalid", func(t *testing.T) {
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodPut, savePath, token,
			`{"name":"x","uid":"","areaId":471,"areaName":"a","address":"b","description":"d","lastDifferent":"2026-03-02T09:30:00+00:00"}`)
	})

	// An application not in Cosmos is a bodyless 404 (no PlanIt fallback).
	t.Run("GET application not found", func(t *testing.T) {
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodGet, "/v1/applications/471/UNKNOWN-"+suffix, token, "")
	})

	// Deleting a non-existent saved row is an idempotent 204 on both.
	t.Run("DELETE saved not found", func(t *testing.T) {
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodDelete, "/v1/me/saved-applications/471/MISSING-"+suffix, token, "")
	})
}

// findSavedEntry returns the saved-list entry with the given applicationUid, or
// nil when absent — so the diff ignores any other rows the user may hold.
func findSavedEntry(t *testing.T, body []byte, uid string) json.RawMessage {
	t.Helper()
	var arr []json.RawMessage
	if err := json.Unmarshal(body, &arr); err != nil {
		t.Fatalf("decode saved list %q: %v", body, err)
	}
	for _, entry := range arr {
		var probe struct {
			ApplicationUID string `json:"applicationUid"`
		}
		if err := json.Unmarshal(entry, &probe); err != nil {
			t.Fatalf("decode saved entry: %v", err)
		}
		if probe.ApplicationUID == uid {
			return entry
		}
	}
	return nil
}
