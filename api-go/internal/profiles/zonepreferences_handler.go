package profiles

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
)

// Per-zone notification preferences live on the user profile document, so the
// /v1/me/watch-zones/{zoneId}/preferences endpoints are served here (over the
// profile store) rather than in the watchzones package. The
// Get/UpdateZonePreferences handlers are registered alongside the watch-zone
// routes even though they read and write the user profile document.

// zonePreferencesRequest is the PUT body. The four channel flags are plain
// bools: an omitted field decodes to false (JSON zero-value decoding for
// non-nullable bool fields).
type zonePreferencesRequest struct {
	NewApplicationPush  bool `json:"newApplicationPush"`
	NewApplicationEmail bool `json:"newApplicationEmail"`
	DecisionPush        bool `json:"decisionPush"`
	DecisionEmail       bool `json:"decisionEmail"`
}

// zonePreferencesResult is the wire response for GET and PUT
// /v1/me/watch-zones/{zoneId}/preferences:
// { zoneId, newApplicationPush, newApplicationEmail, decisionPush, decisionEmail }.
type zonePreferencesResult struct {
	ZoneID              string `json:"zoneId"`
	NewApplicationPush  bool   `json:"newApplicationPush"`
	NewApplicationEmail bool   `json:"newApplicationEmail"`
	DecisionPush        bool   `json:"decisionPush"`
	DecisionEmail       bool   `json:"decisionEmail"`
}

func zonePreferencesResultFrom(zoneID string, prefs ZonePreferences) zonePreferencesResult {
	return zonePreferencesResult{
		ZoneID:              zoneID,
		NewApplicationPush:  prefs.NewApplicationPush,
		NewApplicationEmail: prefs.NewApplicationEmail,
		DecisionPush:        prefs.DecisionPush,
		DecisionEmail:       prefs.DecisionEmail,
	}
}

// getZonePreferences implements GET /v1/me/watch-zones/{zoneId}/preferences. A
// zone the user never customised returns the all-on defaults; a missing profile
// is a bodyless 404.
func (h *handler) getZonePreferences(w http.ResponseWriter, r *http.Request) {
	subject := auth.Subject(r.Context())
	zoneID := r.PathValue("zoneId")

	profile, err := h.store.Get(r.Context(), subject)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		h.serverError(w, r, "load profile", err)
		return
	}
	h.writeJSON(w, r, zonePreferencesResultFrom(zoneID, profile.GetZonePreferences(zoneID)))
}

// putZonePreferences implements PUT /v1/me/watch-zones/{zoneId}/preferences. It
// replaces the zone's stored preferences with the body values and returns them;
// a missing profile is a bodyless 404.
func (h *handler) putZonePreferences(w http.ResponseWriter, r *http.Request) {
	subject := auth.Subject(r.Context())
	zoneID := r.PathValue("zoneId")

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req zonePreferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	profile, err := h.store.Get(r.Context(), subject)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		h.serverError(w, r, "load profile", err)
		return
	}

	// zonePreferencesRequest and ZonePreferences are field-identical; the request
	// type exists only to carry the JSON tags, so a direct conversion is exact.
	prefs := ZonePreferences(req)
	profile.SetZonePreferences(zoneID, prefs)
	if err := h.store.Save(r.Context(), profile); err != nil {
		h.serverError(w, r, "save profile", err)
		return
	}
	h.writeJSON(w, r, zonePreferencesResultFrom(zoneID, prefs))
}
