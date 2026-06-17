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
)

// maxBodyBytes caps the request body the /v1/me write handlers read.
const maxBodyBytes = 1 << 20

// farFutureExpiry is the auto-grant subscription expiry, matching .NET's
// CreateUserProfileCommandHandler.FarFutureExpiry (2099-12-31).
var farFutureExpiry = time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC)

// profileStore is the consumer-side store the handlers use. *CosmosStore
// satisfies it; tests substitute a hand-written fake.
type profileStore interface {
	Get(ctx context.Context, userID string) (*UserProfile, error)
	Save(ctx context.Context, p *UserProfile) error
	Delete(ctx context.Context, userID string) error
}

// handler serves the /v1/me lifecycle. It depends on the profile store, the
// Auth0 management client (real or no-op), the per-container cascade deleters
// account erasure runs, the auto-grant pro-domain list, a clock, and a logger —
// all injected, no globals.
type handler struct {
	store      profileStore
	auth0      Auth0Manager
	cascade    CascadeDeleters
	proDomains proDomainSet
	now        func() time.Time
	logger     *slog.Logger
}

// newHandler builds the /v1/me handler.
func newHandler(store profileStore, auth0 Auth0Manager, proDomains string, cascade CascadeDeleters, now func() time.Time, logger *slog.Logger) *handler {
	return &handler{
		store:      store,
		auth0:      auth0,
		cascade:    cascade,
		proDomains: newProDomainSet(proDomains),
		now:        now,
		logger:     logger,
	}
}

// Routes registers the /v1/me endpoints on mux. All are authenticated: the auth
// middleware guarantees a subject in context before these handlers run.
func Routes(mux *http.ServeMux, store profileStore, auth0 Auth0Manager, proDomains string, cascade CascadeDeleters, now func() time.Time, logger *slog.Logger) {
	h := newHandler(store, auth0, proDomains, cascade, now, logger)
	mux.HandleFunc("POST /v1/me", h.create)
	mux.HandleFunc("GET /v1/me", h.get)
	mux.HandleFunc("PATCH /v1/me", h.patch)
	mux.HandleFunc("DELETE /v1/me", h.delete)
	mux.HandleFunc("GET /v1/me/data", h.exportData)
	// Per-zone notification preferences read/write the profile document, so they
	// are served here (over the profile store) even though their route sits under
	// /me/watch-zones — mirroring .NET's UserProfiles-slice handlers.
	mux.HandleFunc("GET /v1/me/watch-zones/{zoneId}/preferences", h.getZonePreferences)
	mux.HandleFunc("PUT /v1/me/watch-zones/{zoneId}/preferences", h.putZonePreferences)
}

// createResult mirrors .NET CreateUserProfileResult: { userId, pushEnabled, tier }.
type createResult struct {
	UserID      string `json:"userId"`
	PushEnabled bool   `json:"pushEnabled"`
	Tier        string `json:"tier"`
}

// profileResult mirrors .NET Get/UpdateUserProfileResult. DigestDay renders as
// the weekday name ("Wednesday") via weekdayName, matching the web serializer's
// string-enum output.
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
// profile is registered with the Free tier — or Pro when a verified pro-domain
// email auto-grants. Auth0 tier drift is backfilled best-effort.
func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFrom(r.Context())

	existing, err := h.store.Get(r.Context(), claims.Subject)
	switch {
	case err == nil:
		if existing.Email == nil && strings.TrimSpace(claims.Email) != "" {
			existing.BackfillEmail(claims.Email)
			if err := h.store.Save(r.Context(), existing); err != nil {
				h.serverError(w, r, "backfill email", err)
				return
			}
		}
		h.tryBackfillAuth0Tier(r.Context(), existing, claims.SubscriptionTier)
		h.writeJSON(w, r, createResult{
			UserID:      existing.UserID,
			PushEnabled: existing.Preferences.PushEnabled,
			Tier:        existing.Tier.String(),
		})
		return
	case errors.Is(err, ErrNotFound):
		// fall through to registration
	default:
		h.serverError(w, r, "load profile", err)
		return
	}

	profile, err := NewProfile(claims.Subject, claims.Email, h.now())
	if err != nil {
		h.serverError(w, r, "register profile", err)
		return
	}
	if claims.EmailVerified && h.proDomains.contains(claims.Email) {
		profile.ActivateSubscription(TierPro, farFutureExpiry)
	}
	if err := h.store.Save(r.Context(), profile); err != nil {
		h.serverError(w, r, "save profile", err)
		return
	}

	h.writeJSON(w, r, createResult{
		UserID:      profile.UserID,
		PushEnabled: profile.Preferences.PushEnabled,
		Tier:        profile.Tier.String(),
	})
}

// get implements GET /v1/me. A missing profile is a bodyless 404 (the error
// envelope is backfilled by middleware), mirroring .NET's Results.NotFound().
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
	h.writeJSON(w, r, profileResultFrom(profile))
}

// updateRequest is the PATCH /v1/me body. DigestDay accepts either the weekday
// name ("Wednesday") or its integer index, mirroring System.Text.Json with the
// string-enum converter. Omitted fields take the .NET command defaults via
// defaultUpdateRequest.
type updateRequest struct {
	PushEnabled        *bool           `json:"pushEnabled"`
	DigestDay          *digestDayValue `json:"digestDay"`
	EmailDigestEnabled *bool           `json:"emailDigestEnabled"`
	SavedDecisionPush  *bool           `json:"savedDecisionPush"`
	SavedDecisionEmail *bool           `json:"savedDecisionEmail"`
}

// patch implements PATCH /v1/me. Unset body fields take the .NET
// UpdateUserProfileCommand record defaults (pushEnabled true, digestDay Monday,
// the rest true), then the merged preferences replace the profile's.
func (h *handler) patch(w http.ResponseWriter, r *http.Request) {
	subject := auth.Subject(r.Context())

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req updateRequest
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

	profile.UpdatePreferences(req.toPreferences())
	if err := h.store.Save(r.Context(), profile); err != nil {
		h.serverError(w, r, "save profile", err)
		return
	}
	h.writeJSON(w, r, profileResultFrom(profile))
}

// delete implements DELETE /v1/me as a complete UK GDPR Art. 17 erasure. It reads
// first so a missing profile is a 404 before any cascade, then runs the shared
// erasure.Cascade: it erases the user's data from every per-user container, then
// the profile document, then — last — the Auth0 user.
//
// Ordering is the safety contract: child records are removed before the profile,
// so a mid-cascade failure leaves the profile present (GET still 200s) and the
// account retryable by a repeat DELETE rather than half-erased; the Auth0 user is
// deleted last so an Auth0 Management-API failure can never strand un-erased
// Cosmos data. Each cascade store tolerates a 404 on an individual document
// internally, and the Auth0 delete tolerates a 404, so the whole flow is
// idempotent.
//
// The cascade execution is now shared with the dormant-cleanup worker via
// internal/erasure (single source of truth, bead tc-gf0g) so the ordered
// container list lives in exactly one place; the earlier drift (the Go handler
// once deleted only the profile and the Auth0 user, orphaning watch zones, saved
// applications, notifications, device registrations and the notification-state
// watermark — bead tc-qkf2) cannot silently recur. The Get-first 404 check and
// the ordering contract above are unchanged.
func (h *handler) delete(w http.ResponseWriter, r *http.Request) {
	subject := auth.Subject(r.Context())

	if _, err := h.store.Get(r.Context(), subject); err != nil {
		if errors.Is(err, ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		h.serverError(w, r, "load profile", err)
		return
	}

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
	h.writeJSON(w, r, newExportUserData(profile))
}

// tryBackfillAuth0Tier syncs Auth0's subscription_tier to the Cosmos tier when
// the JWT claim drifts. Failures are swallowed and logged — an Auth0 outage must
// never fail POST /v1/me, mirroring .NET's best-effort backfill.
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

func profileResultFrom(p *UserProfile) profileResult {
	return profileResult{
		UserID:             p.UserID,
		PushEnabled:        p.Preferences.PushEnabled,
		DigestDay:          weekdayName(p.Preferences.DigestDay),
		EmailDigestEnabled: p.Preferences.EmailDigestEnabled,
		SavedDecisionPush:  p.Preferences.SavedDecisionPush,
		SavedDecisionEmail: p.Preferences.SavedDecisionEmail,
		Tier:               p.Tier.String(),
	}
}

// writeJSON encodes v as a 200 application/json; charset=utf-8 response with
// HTML escaping off and no trailing newline, matching ASP.NET's Results.Ok byte
// output (the same approach the /api/me handler uses). Every /v1/me success path
// is a 200, so the status is fixed.
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

// serverError logs and emits a bodyless 500; the error envelope (with Detail) is
// backfilled by middleware.ErrorBody, mirroring the rest of the API. The failing
// operation is carried as a structured field so the log message stays constant.
func (h *handler) serverError(w http.ResponseWriter, r *http.Request, op string, err error) {
	h.logger.ErrorContext(r.Context(), "profile request failed", "op", op, "error", err)
	w.WriteHeader(http.StatusInternalServerError)
}

// proDomainSet is the parsed, case-insensitive set of auto-grant pro domains.
type proDomainSet map[string]struct{}

func newProDomainSet(raw string) proDomainSet {
	set := proDomainSet{}
	for _, part := range strings.Split(raw, ",") {
		if d := strings.TrimSpace(part); d != "" {
			set[strings.ToLower(d)] = struct{}{}
		}
	}
	return set
}

// contains reports whether the email's domain is an auto-grant pro domain,
// mirroring .NET AutoGrantOptions.IsProDomain (case-insensitive domain match).
func (s proDomainSet) contains(email string) bool {
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return false
	}
	_, ok := s[strings.ToLower(email[at+1:])]
	return ok
}

// weekdayName serialises a time.Weekday as its English name (e.g. "Wednesday"),
// matching the web serializer's UseStringEnumConverter output for DayOfWeek.
type weekdayName time.Weekday

func (d weekdayName) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Weekday(d).String())
}

// digestDayValue accepts a digest day as either the weekday name ("Wednesday")
// or its integer index (0=Sunday..6=Saturday), mirroring System.Text.Json with
// the string-enum converter on the inbound side.
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

// toPreferences merges the PATCH body onto the .NET command defaults: an absent
// field takes its default (pushEnabled true, digestDay Monday, email/saved flags
// true), so a partial body never silently zeroes an unspecified preference.
func (req updateRequest) toPreferences() NotificationPreferences {
	prefs := NotificationPreferences{
		PushEnabled:        true,
		DigestDay:          time.Monday,
		EmailDigestEnabled: true,
		SavedDecisionPush:  true,
		SavedDecisionEmail: true,
	}
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
