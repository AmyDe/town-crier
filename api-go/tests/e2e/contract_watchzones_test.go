//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// Valid-token contract scenarios for the iteration-5 watch-zone surface. The
// shipped surface is GET list, PATCH update, DELETE, and GET/PUT per-zone
// preferences; POST create and GET /{zoneId}/applications are deferred (tc-5847)
// and so a zone is created via the .NET API as un-diffed setup. Both APIs share
// one Cosmos database, so every diffed call observes the same deterministic
// state and body equality is exact.
//
// Quota-safe: exactly one zone is created and it is always deleted on cleanup
// (only the zone this test created is touched). Scenarios skip when the token
// env is absent so plain local runs stay green.

// wzExchange is one authenticated request that may carry a JSON body; it also
// captures the Location header (used to read a created zone's id).
type wzExchange struct {
	status      int
	contentType string
	location    string
	body        []byte
}

func watchZoneRequest(t *testing.T, client *http.Client, base, method, path, token, body string) wzExchange {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, reader)
	if err != nil {
		t.Fatalf("new request %s %s: %v", method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		t.Fatalf("read body %s %s: %v", method, path, err)
	}
	return wzExchange{
		status:      resp.StatusCode,
		contentType: resp.Header.Get("Content-Type"),
		location:    resp.Header.Get("Location"),
		body:        raw,
	}
}

// diffWatchZone runs the same authenticated request against both APIs and
// asserts status, content type, and JSON body match (the .NET response is the
// source of truth). Bodyless responses are compared as raw bytes.
func diffWatchZone(t *testing.T, client *http.Client, dotnetURL, goURL, method, path, token, body string) {
	t.Helper()

	want := watchZoneRequest(t, client, dotnetURL, method, path, token, body)
	got := watchZoneRequest(t, client, goURL, method, path, token, body)

	if got.status != want.status {
		t.Errorf("%s %s status: go=%d dotnet=%d (go body %s)", method, path, got.status, want.status, got.body)
	}
	if got.contentType != want.contentType {
		t.Errorf("%s %s content-type: go=%q dotnet=%q", method, path, got.contentType, want.contentType)
	}
	if len(want.body) == 0 || len(got.body) == 0 {
		if !bytes.Equal(got.body, want.body) {
			t.Errorf("%s %s body: go=%q dotnet=%q", method, path, got.body, want.body)
		}
		return
	}
	if !jsonEqual(t, got.body, want.body) {
		t.Errorf("%s %s body: go=%s dotnet=%s", method, path, got.body, want.body)
	}
}

func TestContract_WatchZones(t *testing.T) {
	dotnetURL := baseURL(t, "DOTNET_BASE_URL")
	goURL := baseURL(t, "GO_BASE_URL")
	token := integrationToken(t)
	client := &http.Client{Timeout: requestTimeout}

	// Setup, not a diff: ensure the profile exists (watch zones require one),
	// then create a single zone on the .NET side. Its id comes back in the
	// Location header (/v1/me/watch-zones/{zoneId}).
	if setup := watchZoneRequest(t, client, dotnetURL, http.MethodPost, "/v1/me", token, ""); setup.status >= 500 {
		t.Fatalf("setup POST /v1/me on .NET: %d %s", setup.status, setup.body)
	}

	suffix := newZoneSuffix(t)
	createBody := fmt.Sprintf(
		`{"name":"Contract Zone %s","latitude":51.5074,"longitude":-0.1278,"radiusMetres":1000,"authorityId":471}`,
		suffix)
	created := watchZoneRequest(t, client, dotnetURL, http.MethodPost, "/v1/me/watch-zones", token, createBody)
	if created.status == http.StatusForbidden {
		t.Skipf("watch-zone quota exhausted on the shared test user — skipping (%s)", created.body)
	}
	if created.status != http.StatusCreated {
		t.Fatalf("setup create on .NET: status %d body %s", created.status, created.body)
	}
	zoneID := created.location[strings.LastIndex(created.location, "/")+1:]
	if zoneID == "" {
		t.Fatalf("setup create returned no zone id in Location %q", created.location)
	}
	zonePath := "/v1/me/watch-zones/" + zoneID
	prefsPath := zonePath + "/preferences"

	// Always remove the created zone, even if an assertion fails.
	t.Cleanup(func() {
		_ = watchZoneRequest(t, client, dotnetURL, http.MethodDelete, zonePath, token, "")
	})

	// List: both read the shared Cosmos partition, so the zone (and any others)
	// appears identically.
	t.Run("GET list", func(t *testing.T) {
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodGet, "/v1/me/watch-zones", token, "")
	})

	// POST create: create the same-geometry zone on the Go API (a distinct name
	// satisfies the per-user unique-name key) and diff the create RESPONSE
	// against the .NET setup create above (the source of truth). Both query the
	// same Applications partition for identical geometry, so the
	// nearbyApplications arrays — the raw-domain wire shape — match.
	t.Run("POST create response", func(t *testing.T) {
		goBody := fmt.Sprintf(
			`{"name":"Contract Zone %s Go","latitude":51.5074,"longitude":-0.1278,"radiusMetres":1000,"authorityId":471}`,
			suffix)
		goCreated := watchZoneRequest(t, client, goURL, http.MethodPost, "/v1/me/watch-zones", token, goBody)
		if goCreated.status == http.StatusForbidden {
			t.Skipf("watch-zone quota exhausted on the shared test user — skipping (%s)", goCreated.body)
		}
		if goCreated.status != http.StatusCreated {
			t.Fatalf("go create: status %d body %s", goCreated.status, goCreated.body)
		}
		goZoneID := goCreated.location[strings.LastIndex(goCreated.location, "/")+1:]
		t.Cleanup(func() {
			_ = watchZoneRequest(t, client, goURL, http.MethodDelete, "/v1/me/watch-zones/"+goZoneID, token, "")
		})
		if goCreated.contentType != created.contentType {
			t.Errorf("create content-type: go=%q dotnet=%q", goCreated.contentType, created.contentType)
		}
		if !jsonEqual(t, goCreated.body, created.body) {
			t.Errorf("create body: go=%s dotnet=%s", goCreated.body, created.body)
		}
	})

	// GET applications: both read the shared zone and its nearby applications, so
	// the list — including the null latestUnreadEvent on each row for a user with
	// no notifications — is identical.
	t.Run("GET applications", func(t *testing.T) {
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodGet, zonePath+"/applications", token, "")
	})

	// Update: a full PATCH body is fully deterministic, so applying it to both
	// APIs (each over the same document) yields identical updated summaries.
	t.Run("PATCH update", func(t *testing.T) {
		patchBody := fmt.Sprintf(
			`{"name":"Contract Zone %s Updated","latitude":52.4862,"longitude":-1.8904,"radiusMetres":1500,"authorityId":471,"pushEnabled":false,"emailInstantEnabled":false}`,
			suffix)
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodPatch, zonePath, token, patchBody)
	})

	// Preferences default to all-on for a zone the user never customised.
	t.Run("GET preferences (defaults)", func(t *testing.T) {
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodGet, prefsPath, token, "")
	})

	// Update preferences: deterministic body, identical result on both.
	t.Run("PUT preferences", func(t *testing.T) {
		prefsBody := `{"newApplicationPush":false,"newApplicationEmail":true,"decisionPush":false,"decisionEmail":true}`
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodPut, prefsPath, token, prefsBody)
	})

	// Read-back reflects the PUT identically on both.
	t.Run("GET preferences (after PUT)", func(t *testing.T) {
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodGet, prefsPath, token, "")
	})

	// A present-but-out-of-range field is a 400 with the ApiErrorResponse
	// envelope; the guard runs before the zone is loaded, so any id works.
	t.Run("PATCH invalid payload", func(t *testing.T) {
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodPatch, zonePath, token, `{"authorityId":0}`)
	})

	// Deleting a non-existent zone is a bodyless 404 on both — exercised on a
	// random id so it never touches the zone this test created.
	t.Run("DELETE not found", func(t *testing.T) {
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodDelete, "/v1/me/watch-zones/"+newZoneSuffix(t), token, "")
	})
}

// newZoneSuffix returns a random hex suffix for unique zone names / ids. The
// WatchZones container enforces a per-user unique key on name, so each created
// zone needs a distinct name even across overlapping CI runs.
func newZoneSuffix(t *testing.T) string {
	t.Helper()
	var b [8]byte
	if _, err := io.ReadFull(rand.Reader, b[:]); err != nil {
		t.Fatalf("random suffix: %v", err)
	}
	return fmt.Sprintf("%x", b)
}
