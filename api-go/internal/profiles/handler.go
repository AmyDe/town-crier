package profiles

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/erasure"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// maxBodyBytes caps the request body the /v1/me write handlers read.
const maxBodyBytes = 1 << 20

// profileStore is the consumer-side store the handlers use.
type profileStore interface {
	Get(ctx context.Context, userID string) (*UserProfile, error)
	Save(ctx context.Context, p *UserProfile) error
	Delete(ctx context.Context, userID string) error
}

// handler serves the /v1/me lifecycle.
type handler struct {
	store         profileStore
	auth0         Auth0Manager
	cascade       CascadeDeleters
	exportReaders ExportReaders
	now           func() time.Time
	logger        *slog.Logger
}

// newHandler builds the /v1/me handler.
func newHandler(store profileStore, auth0 Auth0Manager, cascade CascadeDeleters, exportReaders ExportReaders, now func() time.Time, logger *slog.Logger) *handler {
	return &handler{
		store:         store,
		auth0:         auth0,
		cascade:       cascade,
		exportReaders: exportReaders,
		now:           now,
		logger:        logger,
	}
}

// Routes registers the /v1/me endpoints on mux. All are authenticated: the auth
// middleware guarantees a subject in context before these handlers run.
func Routes(mux *http.ServeMux, store profileStore, auth0 Auth0Manager, cascade CascadeDeleters, exportReaders ExportReaders, now func() time.Time, logger *slog.Logger) {
	h := newHandler(store, auth0, cascade, exportReaders, now, logger)
	mux.HandleFunc("POST /v1/me", h.create)
	mux.HandleFunc("GET /v1/me", h.get)
	mux.HandleFunc("PATCH /v1/me", h.patch)
	mux.HandleFunc("DELETE /v1/me", h.delete)
	mux.HandleFunc("GET /v1/me/data", h.exportData)
	// Zone preferences live on the profile, so they are served here despite the
	// /me/watch-zones route.
	mux.HandleFunc("GET /v1/me/watch-zones/{zoneId}/preferences", h.getZonePreferences)
	mux.HandleFunc("PUT /v1/me/watch-zones/{zoneId}/preferences", h.putZonePreferences)
}

// createResult is the POST /v1/me response body: { userId, pushEnabled, tier }.
type createResult struct {
	UserID      string `json:"userId"`
	PushEnabled bool   `json:"pushEnabled"`
	Tier        string `json:"tier"`
}

// profileResult is the GET and PATCH /v1/me response body. DigestDay renders as
// the weekday name ("Wednesday") via weekdayName.
type profileResult struct {
	UserID             string      `json:"userId"`
	PushEnabled        bool        `json:"pushEnabled"`
	DigestDay          weekdayName `json:"digestDay"`
	EmailDigestEnabled bool        `json:"emailDigestEnabled"`
	SavedDecisionPush  bool        `json:"savedDecisionPush"`
	SavedDecisionEmail bool        `json:"savedDecisionEmail"`
	Tier               string      `json:"tier"`
}

// create implements POST /v1/me. It is idempotent: an existing profile is
// returned unchanged (with an email backfill when newly available), and a fresh
// profile is registered with the Free tier. Auth0 tier drift is backfilled
// best-effort.
func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFrom(r.Context())

	var profile *UserProfile
	existed := false
	err := RetryOnConflict(r.Context(), DefaultSaveAttempts, func(ctx context.Context) error {
		existed = false
		existing, err := h.store.Get(ctx, claims.Subject)
		switch {
		case err == nil:
			if existing.Email == nil && strings.TrimSpace(claims.Email) != "" {
				existing.BackfillEmail(claims.Email)
				if err := h.store.Save(ctx, existing); err != nil {
					return fmt.Errorf("backfill email: %w", err)
				}
			}
			profile = existing
			existed = true
			return nil
		case errors.Is(err, ErrNotFound):
		default:
			return fmt.Errorf("load profile: %w", err)
		}

		fresh, err := NewProfile(claims.Subject, claims.Email, h.now())
		if err != nil {
			return fmt.Errorf("register profile: %w", err)
		}
		if err := h.store.Save(ctx, fresh); err != nil {
			return fmt.Errorf("save profile: %w", err)
		}
		profile = fresh
		return nil
	})
	if err != nil {
		h.serverError(w, r, "create profile", err)
		return
	}

	if existed {
		h.tryBackfillAuth0Tier(r.Context(), profile, claims.SubscriptionTier)
	}
	h.writeJSON(w, r, createResult{
		UserID:      profile.UserID,
		PushEnabled: profile.Preferences.PushEnabled,
		Tier:        profile.EffectiveTier(h.now()).String(),
	})
}

// get implements GET /v1/me. A missing profile is a bodyless 404 (the error
// envelope is backfilled by middleware).
func (h *handler) get(w http.ResponseWriter, r *http.Request) {
	subject := auth.Subject(r.Context())
	profile, err := h.store.Get(r.Context(), subject)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		h.serverError(w, r, "load profile", err)
		return
	}
	h.writeJSON(w, r, profileResultFrom(profile, h.now()))
}

// updateRequest is the PATCH /v1/me body. DigestDay accepts either the weekday
// name ("Wednesday") or its integer index. Omitted fields take their command
// defaults via toPreferences.
type updateRequest struct {
	PushEnabled        *bool           `json:"pushEnabled"`
	DigestDay          *digestDayValue `json:"digestDay"`
	EmailDigestEnabled *bool           `json:"emailDigestEnabled"`
	SavedDecisionPush  *bool           `json:"savedDecisionPush"`
	SavedDecisionEmail *bool           `json:"savedDecisionEmail"`
}

// patch implements PATCH /v1/me. Unset body fields take their command defaults
// (pushEnabled true, digestDay Monday, the rest true), then the merged
// preferences replace the profile's.
func (h *handler) patch(w http.ResponseWriter, r *http.Request) {
	subject := auth.Subject(r.Context())

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var profile *UserProfile
	err := RetryOnConflict(r.Context(), DefaultSaveAttempts, func(ctx context.Context) error {
		p, err := h.store.Get(ctx, subject)
		if err != nil {
			return err
		}
		p.UpdatePreferences(req.toPreferences())
		profile = p
		return h.store.Save(ctx, p)
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		h.serverError(w, r, "save profile", err)
		return
	}
	h.writeJSON(w, r, profileResultFrom(profile, h.now()))
}

// delete implements DELETE /v1/me as a complete UK GDPR Art. 17 erasure. The
// cascade always runs, so a repeat call finishes an erasure that failed after
// the profile was removed; erasure.Cascade owns the ordering and idempotency
// contract.
func (h *handler) delete(w http.ResponseWriter, r *http.Request) {
	subject := auth.Subject(r.Context())

	deleters := erasure.Deleters{
		Notifications:       h.cascade.Notifications,
		WatchZones:          h.cascade.WatchZones,
		SavedApplications:   h.cascade.SavedApplications,
		DeviceRegistrations: h.cascade.DeviceRegistrations,
		NotificationState:   h.cascade.NotificationState,
		OfferCodes:          h.cascade.OfferCodes,
		Profile:             h.store,
		Auth0:               h.auth0,
		ProfileAbsent:       func(e error) bool { return errors.Is(e, ErrNotFound) },
	}
	if err := erasure.Cascade(r.Context(), subject, deleters); err != nil {
		h.serverError(w, r, "erase account", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// exportData implements GET /v1/me/data, the GDPR export. A missing profile is
// a bodyless 404.
func (h *handler) exportData(w http.ResponseWriter, r *http.Request) {
	subject := auth.Subject(r.Context())
	profile, err := h.store.Get(r.Context(), subject)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		h.serverError(w, r, "load profile", err)
		return
	}
	export, err := newExportUserData(r.Context(), profile, h.exportReaders)
	if err != nil {
		h.serverError(w, r, "build export", err)
		return
	}
	h.writeJSON(w, r, export)
}

// tryBackfillAuth0Tier syncs Auth0's subscription_tier to the stored tier when
// the JWT claim drifts. Failures are swallowed and logged — an Auth0 outage must
// never fail POST /v1/me.
func (h *handler) tryBackfillAuth0Tier(ctx context.Context, p *UserProfile, jwtTier string) {
	if strings.TrimSpace(jwtTier) == "" {
		return
	}
	cosmosTier := p.Tier.String()
	if strings.EqualFold(cosmosTier, jwtTier) {
		return
	}
	if err := h.auth0.UpdateSubscriptionTier(ctx, p.UserID, cosmosTier); err != nil {
		h.logger.WarnContext(ctx, "auth0 tier backfill failed; will retry on next POST /v1/me",
			"userId", p.UserID, "error", err)
	}
}

func profileResultFrom(p *UserProfile, now time.Time) profileResult {
	return profileResult{
		UserID:             p.UserID,
		PushEnabled:        p.Preferences.PushEnabled,
		DigestDay:          weekdayName(p.Preferences.DigestDay),
		EmailDigestEnabled: p.Preferences.EmailDigestEnabled,
		SavedDecisionPush:  p.Preferences.SavedDecisionPush,
		SavedDecisionEmail: p.Preferences.SavedDecisionEmail,
		Tier:               p.EffectiveTier(now).String(),
	}
}

// writeJSON encodes v as a 200 application/json; charset=utf-8 response with
// HTML escaping off and no trailing newline. Every /v1/me success path is a 200,
// so the status is fixed.
func (h *handler) writeJSON(w http.ResponseWriter, r *http.Request, v any) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		h.serverError(w, r, "encode response", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(bytes.TrimRight(buf.Bytes(), "\n")); err != nil {
		h.logger.ErrorContext(r.Context(), "write response", "error", err)
	}
}

// serverError answers a bodyless error status chosen by platform.ClassifyRequestError
// (499 unlogged for a cancelled request, 503 for a timeout, otherwise 500).
func (h *handler) serverError(w http.ResponseWriter, r *http.Request, op string, err error) {
	platform.RespondRequestError(w, r, h.logger, "profile request failed", err, "op", op)
}

// weekdayName serialises a time.Weekday as its English name (e.g. "Wednesday"),
// matching the web API's string-enum convention.
type weekdayName time.Weekday

func (d weekdayName) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Weekday(d).String())
}

// digestDayValue accepts a digest day as either the weekday name ("Wednesday")
// or its integer index (0=Sunday..6=Saturday) on the inbound side.
type digestDayValue time.Weekday

func (d *digestDayValue) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) > 0 && data[0] == '"' {
		var name string
		if err := json.Unmarshal(data, &name); err != nil {
			return err
		}
		wd, err := parseWeekday(name)
		if err != nil {
			return err
		}
		*d = digestDayValue(wd)
		return nil
	}
	var n int
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	if n < int(time.Sunday) || n > int(time.Saturday) {
		return fmt.Errorf("digestDay out of range: %d", n)
	}
	*d = digestDayValue(time.Weekday(n))
	return nil
}

func parseWeekday(name string) (time.Weekday, error) {
	for d := time.Sunday; d <= time.Saturday; d++ {
		if strings.EqualFold(d.String(), name) {
			return d, nil
		}
	}
	return time.Sunday, fmt.Errorf("unknown weekday %q", name)
}

// toPreferences merges the PATCH body onto the command defaults: an absent field
// takes its default (pushEnabled true, digestDay Monday, email/saved flags true),
// so a partial body never silently zeroes an unspecified preference.
func (req updateRequest) toPreferences() NotificationPreferences {
	prefs := DefaultPreferences()
	if req.PushEnabled != nil {
		prefs.PushEnabled = *req.PushEnabled
	}
	if req.DigestDay != nil {
		prefs.DigestDay = time.Weekday(*req.DigestDay)
	}
	if req.EmailDigestEnabled != nil {
		prefs.EmailDigestEnabled = *req.EmailDigestEnabled
	}
	if req.SavedDecisionPush != nil {
		prefs.SavedDecisionPush = *req.SavedDecisionPush
	}
	if req.SavedDecisionEmail != nil {
		prefs.SavedDecisionEmail = *req.SavedDecisionEmail
	}
	return prefs
}
