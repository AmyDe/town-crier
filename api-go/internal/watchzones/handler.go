package watchzones

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
)

// maxBodyBytes caps the request body the PATCH handler reads.
const maxBodyBytes = 1 << 20

// invalidPayloadMessage is the .NET ApiErrorResponse error text for a watch-zone
// validation failure (Create/Update endpoints).
const invalidPayloadMessage = "Invalid watch zone payload."

// zoneStore is the consumer-side store the handlers use. *CosmosStore satisfies
// it; tests substitute a hand-written fake.
type zoneStore interface {
	GetByUserID(ctx context.Context, userID string) ([]WatchZone, error)
	Get(ctx context.Context, userID, zoneID string) (WatchZone, error)
	Save(ctx context.Context, z WatchZone) error
	Delete(ctx context.Context, userID, zoneID string) error
}

// handler serves the /v1/me/watch-zones list/update/delete surface. The auth
// middleware guarantees a subject in context before these handlers run.
type handler struct {
	store  zoneStore
	logger *slog.Logger
}

// Routes registers the in-scope watch-zone endpoints on mux. POST create and
// GET /{zoneId}/applications are intentionally absent (deferred to tc-5847).
func Routes(mux *http.ServeMux, store zoneStore, logger *slog.Logger) {
	h := &handler{store: store, logger: logger}
	mux.HandleFunc("GET /v1/me/watch-zones", h.list)
	mux.HandleFunc("PATCH /v1/me/watch-zones/{zoneId}", h.patch)
	mux.HandleFunc("DELETE /v1/me/watch-zones/{zoneId}", h.delete)
}

// watchZoneSummary mirrors .NET WatchZoneSummary: the per-zone shape returned by
// list and update. createdAt is deliberately absent, matching .NET.
type watchZoneSummary struct {
	ID                  string  `json:"id"`
	Name                string  `json:"name"`
	Latitude            float64 `json:"latitude"`
	Longitude           float64 `json:"longitude"`
	RadiusMetres        float64 `json:"radiusMetres"`
	AuthorityID         int     `json:"authorityId"`
	PushEnabled         bool    `json:"pushEnabled"`
	EmailInstantEnabled bool    `json:"emailInstantEnabled"`
}

func summaryOf(z WatchZone) watchZoneSummary {
	return watchZoneSummary{
		ID:                  z.ID,
		Name:                z.Name,
		Latitude:            z.Latitude,
		Longitude:           z.Longitude,
		RadiusMetres:        z.RadiusMetres,
		AuthorityID:         z.AuthorityID,
		PushEnabled:         z.PushEnabled,
		EmailInstantEnabled: z.EmailInstantEnabled,
	}
}

// listResult mirrors .NET ListWatchZonesResult: { zones: [...] }.
type listResult struct {
	Zones []watchZoneSummary `json:"zones"`
}

// updateResult mirrors .NET UpdateWatchZoneResult: { zone: {...} }.
type updateResult struct {
	Zone watchZoneSummary `json:"zone"`
}

// list implements GET /v1/me/watch-zones, returning the user's zones as a
// (possibly empty) array. Always a 200.
func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())
	zones, err := h.store.GetByUserID(r.Context(), userID)
	if err != nil {
		h.serverError(w, r, "list watch zones", err)
		return
	}
	summaries := make([]watchZoneSummary, 0, len(zones))
	for _, z := range zones {
		summaries = append(summaries, summaryOf(z))
	}
	h.writeJSON(w, r, http.StatusOK, listResult{Zones: summaries})
}

// patchRequest is the PATCH body: every field optional (nil = unchanged),
// mirroring .NET UpdateWatchZoneRequest.
type patchRequest struct {
	Name                *string  `json:"name"`
	Latitude            *float64 `json:"latitude"`
	Longitude           *float64 `json:"longitude"`
	RadiusMetres        *float64 `json:"radiusMetres"`
	AuthorityID         *int     `json:"authorityId"`
	PushEnabled         *bool    `json:"pushEnabled"`
	EmailInstantEnabled *bool    `json:"emailInstantEnabled"`
}

// rangeValid reports whether the present coordinate/radius/authority fields are
// in range, mirroring the .NET endpoint's pre-handler guard. A nil field is not
// checked. Name is deliberately not checked here — like .NET, that is enforced
// by the domain merge (and a blank name there is a 500, not a 400).
func (req patchRequest) rangeValid() bool {
	if req.Latitude != nil && (*req.Latitude < -90 || *req.Latitude > 90) {
		return false
	}
	if req.Longitude != nil && (*req.Longitude < -180 || *req.Longitude > 180) {
		return false
	}
	if req.RadiusMetres != nil && *req.RadiusMetres <= 0 {
		return false
	}
	if req.AuthorityID != nil && *req.AuthorityID <= 0 {
		return false
	}
	return true
}

func (req patchRequest) toUpdate() ZoneUpdate {
	// patchRequest and ZoneUpdate are field-identical; the request type exists
	// only to carry the JSON tags, so a direct conversion is exact.
	return ZoneUpdate(req)
}

// patch implements PATCH /v1/me/watch-zones/{zoneId}: range-validate the body
// (400), load the zone (404 if absent), apply the merge, persist, and return the
// updated summary. A merge that violates a domain invariant (e.g. a blank name)
// is a 500, mirroring .NET's unhandled ArgumentException.
func (h *handler) patch(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())
	zoneID := r.PathValue("zoneId")

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req patchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !req.rangeValid() {
		h.writeError(w, r, http.StatusBadRequest, invalidPayloadMessage)
		return
	}

	zone, err := h.store.Get(r.Context(), userID, zoneID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		h.serverError(w, r, "load watch zone", err)
		return
	}

	updated, err := zone.WithUpdates(req.toUpdate())
	if err != nil {
		h.serverError(w, r, "apply watch-zone update", err)
		return
	}
	if err := h.store.Save(r.Context(), updated); err != nil {
		h.serverError(w, r, "save watch zone", err)
		return
	}
	h.writeJSON(w, r, http.StatusOK, updateResult{Zone: summaryOf(updated)})
}

// delete implements DELETE /v1/me/watch-zones/{zoneId}: 204 on success, 404 when
// the zone does not exist.
func (h *handler) delete(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())
	zoneID := r.PathValue("zoneId")

	if err := h.store.Delete(r.Context(), userID, zoneID); err != nil {
		if errors.Is(err, ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		h.serverError(w, r, "delete watch zone", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// apiErrorResponse mirrors the .NET ApiErrorResponse: { error, message } with
// message serialised as an explicit null when unset.
type apiErrorResponse struct {
	Error   string  `json:"error"`
	Message *string `json:"message"`
}

// writeJSON encodes v as an application/json; charset=utf-8 response with HTML
// escaping off and no trailing newline, matching ASP.NET's Results.Ok byte
// output (the same approach the profiles handler uses).
func (h *handler) writeJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	body, err := encodeJSON(v)
	if err != nil {
		h.serverError(w, r, "encode response", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		h.logger.ErrorContext(r.Context(), "write response", "error", err)
	}
}

// writeError emits the .NET ApiErrorResponse envelope at the given status with
// the same content type as a success body (Results.Json defaults to
// application/json; charset=utf-8).
func (h *handler) writeError(w http.ResponseWriter, r *http.Request, status int, message string) {
	body, err := encodeJSON(apiErrorResponse{Error: message})
	if err != nil {
		h.serverError(w, r, "encode error", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		h.logger.ErrorContext(r.Context(), "write error body", "error", err)
	}
}

// encodeJSON renders v with HTML escaping disabled and the trailing newline
// trimmed, matching the relaxed-encoder byte output of the .NET web serializer.
func encodeJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// serverError logs and emits a bodyless 500; the error envelope (with Detail) is
// backfilled by middleware.ErrorBody, mirroring the rest of the API.
func (h *handler) serverError(w http.ResponseWriter, r *http.Request, op string, err error) {
	h.logger.ErrorContext(r.Context(), "watch-zone request failed", "op", op, "error", err)
	w.WriteHeader(http.StatusInternalServerError)
}
