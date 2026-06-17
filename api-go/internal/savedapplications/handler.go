package savedapplications

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/httputil"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

const maxBodyBytes = 1 << 20

// invalidBodyMessage is the .NET ApiErrorResponse text when the save body lacks
// the fields needed to build the canonical key and master record.
const invalidBodyMessage = "Body must include a non-empty uid and name."

// savedStore is the consumer-side saved-application store.
type savedStore interface {
	Save(ctx context.Context, sa SavedApplication) error
	Exists(ctx context.Context, userID, applicationUID string) (bool, error)
	Delete(ctx context.Context, userID, applicationUID string) error
	GetByUserID(ctx context.Context, userID string) ([]SavedApplication, error)
}

// appStore is the consumer-side planning-application store the saved handler
// needs: an upsert so a save always points at a known master record (the search
// path no longer upserts), and a partition-scoped uid lookup used by the lazy
// snapshot backfill for legacy rows.
type appStore interface {
	Upsert(ctx context.Context, a applications.PlanningApplication) error
	GetByUID(ctx context.Context, uid, authorityCode string) (applications.PlanningApplication, bool, error)
}

type handler struct {
	store  savedStore
	apps   appStore
	now    func() time.Time
	logger *slog.Logger
}

// Routes registers the saved-application endpoints. PUT/DELETE use a {**uid}
// catch-all so a slash-bearing application uid is captured whole, mirroring the
// .NET {**applicationUid} route.
func Routes(mux *http.ServeMux, store savedStore, apps appStore, now func() time.Time, logger *slog.Logger) {
	h := &handler{store: store, apps: apps, now: now, logger: logger}
	mux.HandleFunc("PUT /v1/me/saved-applications/{applicationUid...}", h.save)
	mux.HandleFunc("DELETE /v1/me/saved-applications/{applicationUid...}", h.delete)
	mux.HandleFunc("GET /v1/me/saved-applications", h.list)
}

// saveRequest is the PUT body — the full planning-application payload. The path
// uid is ignored; the saved record's identity is the canonical uid derived from
// the body (areaId/name).
type saveRequest struct {
	Name          string              `json:"name"`
	UID           string              `json:"uid"`
	AreaName      string              `json:"areaName"`
	AreaID        int                 `json:"areaId"`
	Address       string              `json:"address"`
	Postcode      *string             `json:"postcode"`
	Description   string              `json:"description"`
	AppType       *string             `json:"appType"`
	AppState      *string             `json:"appState"`
	AppSize       *string             `json:"appSize"`
	StartDate     *platform.DateOnly  `json:"startDate"`
	DecidedDate   *platform.DateOnly  `json:"decidedDate"`
	ConsultedDate *platform.DateOnly  `json:"consultedDate"`
	Longitude     *float64            `json:"longitude"`
	Latitude      *float64            `json:"latitude"`
	URL           *string             `json:"url"`
	Link          *string             `json:"link"`
	LastDifferent platform.DotNetTime `json:"lastDifferent"`
}

func (req saveRequest) toApplication() applications.PlanningApplication {
	return applications.PlanningApplication{
		Name:          req.Name,
		UID:           req.UID,
		AreaName:      req.AreaName,
		AreaID:        req.AreaID,
		Address:       req.Address,
		Postcode:      req.Postcode,
		Description:   req.Description,
		AppType:       req.AppType,
		AppState:      req.AppState,
		AppSize:       req.AppSize,
		StartDate:     dateToTime(req.StartDate),
		DecidedDate:   dateToTime(req.DecidedDate),
		ConsultedDate: dateToTime(req.ConsultedDate),
		Longitude:     req.Longitude,
		Latitude:      req.Latitude,
		URL:           req.URL,
		Link:          req.Link,
		LastDifferent: time.Time(req.LastDifferent),
	}
}

// save implements PUT /v1/me/saved-applications/{**uid}. The path uid is
// ignored. The body must carry a non-blank uid and name; the master record is
// upserted first, then the saved row is written keyed on the canonical uid
// (skipped when one already exists — idempotent re-save).
func (h *handler) save(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req saveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.UID) == "" || strings.TrimSpace(req.Name) == "" {
		h.writeError(w, r, http.StatusBadRequest, invalidBodyMessage)
		return
	}

	app := req.toApplication()
	if err := h.apps.Upsert(r.Context(), app); err != nil {
		h.serverError(w, r, "upsert application", err)
		return
	}

	canonicalUID := app.CanonicalUID()
	exists, err := h.store.Exists(r.Context(), userID, canonicalUID)
	if err != nil {
		h.serverError(w, r, "check saved application", err)
		return
	}
	if !exists {
		if err := h.store.Save(r.Context(), NewSavedApplication(userID, app, h.now())); err != nil {
			h.serverError(w, r, "save application", err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// delete implements DELETE /v1/me/saved-applications/{**uid}. The delete is
// idempotent — a missing record still returns 204.
func (h *handler) delete(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())
	applicationUID := r.PathValue("applicationUid")

	if err := h.store.Delete(r.Context(), userID, applicationUID); err != nil {
		h.serverError(w, r, "delete saved application", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// savedEntry mirrors .NET SavedApplicationResult: { applicationUid, savedAt,
// application }.
type savedEntry struct {
	ApplicationUID string              `json:"applicationUid"`
	SavedAt        platform.DotNetTime `json:"savedAt"`
	Application    applications.Result `json:"application"`
}

// list implements GET /v1/me/saved-applications, returning a JSON array of the
// user's saved applications rendered from their embedded snapshots. It runs the
// lazy migration the .NET GetSavedApplicationsQueryHandler runs on every read,
// reachable only by pre-PR#398 legacy data: (1) backfill the snapshot for rows
// persisted before the snapshot column existed, (2) re-key legacy bare-ref uids
// to the canonical {areaId}/{name} uid, (3) dedup a legacy+canonical pair for
// the same application to a single row.
func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())

	saved, err := h.store.GetByUserID(r.Context(), userID)
	if err != nil {
		h.serverError(w, r, "list saved applications", err)
		return
	}

	// Track the canonical uids already emitted this read so a legacy+canonical
	// duplicate pair for the same app collapses to a single row.
	emitted := make(map[string]struct{}, len(saved))
	entries := make([]savedEntry, 0, len(saved))
	for _, record := range saved {
		hydrated, ok, err := h.hydrate(r.Context(), record)
		if err != nil {
			h.serverError(w, r, "hydrate saved application", err)
			return
		}
		if !ok {
			// Master record gone — exclude rather than failing the whole list.
			continue
		}

		canonical := hydrated
		if isLegacyKeyed(hydrated) {
			canonical, err = h.reKeyToCanonical(r.Context(), hydrated)
			if err != nil {
				h.serverError(w, r, "re-key saved application", err)
				return
			}
		}

		if _, seen := emitted[canonical.ApplicationUID]; seen {
			continue
		}
		emitted[canonical.ApplicationUID] = struct{}{}
		entries = append(entries, savedEntry{
			ApplicationUID: canonical.ApplicationUID,
			SavedAt:        platform.DotNetTime(canonical.SavedAt),
			Application:    applications.ResultOf(*canonical.Application),
		})
	}
	h.writeJSON(w, r, entries)
}

// isLegacyKeyed reports whether a row is keyed on a legacy bare-ref uid rather
// than the canonical {areaId}/{name} uid. Only decidable once the snapshot is
// embedded — the canonical uid is derived from the snapshot.
func isLegacyKeyed(record SavedApplication) bool {
	return record.Application != nil && record.ApplicationUID != record.Application.CanonicalUID()
}

// hydrate ensures the saved record carries an embedded snapshot. Rows persisted
// before the snapshot column existed hold only the uid; they are backfilled once
// via the partition-scoped planning lookup and rewritten in place so subsequent
// reads are zero-hydration. The bool is false when the master planning
// application is gone (the row is excluded). The row's existing ApplicationUID is
// preserved — re-keying happens separately so the two steps stay independent.
func (h *handler) hydrate(ctx context.Context, record SavedApplication) (SavedApplication, bool, error) {
	if record.Application != nil {
		return record, true, nil
	}

	authorityCode := strconv.Itoa(record.AuthorityID)
	fetched, found, err := h.apps.GetByUID(ctx, record.ApplicationUID, authorityCode)
	if err != nil {
		return SavedApplication{}, false, err
	}
	if !found {
		return SavedApplication{}, false, nil
	}

	refreshed := record.withEmbeddedSnapshot(fetched)
	if err := h.store.Save(ctx, refreshed); err != nil {
		return SavedApplication{}, false, err
	}
	return refreshed, true, nil
}

// reKeyToCanonical re-keys a legacy-format saved row to the canonical
// {areaId}/{name} uid. Cosmos doc ids are immutable, so a re-key is a write of
// the canonical doc plus a delete of the legacy doc. When a canonical doc already
// exists for the same user+app (the confirmed legacy+canonical duplicate case)
// the canonical doc is kept untouched and only the legacy doc is deleted.
func (h *handler) reKeyToCanonical(ctx context.Context, legacy SavedApplication) (SavedApplication, error) {
	canonical := NewSavedApplication(legacy.UserID, *legacy.Application, legacy.SavedAt)

	exists, err := h.store.Exists(ctx, canonical.UserID, canonical.ApplicationUID)
	if err != nil {
		return SavedApplication{}, err
	}
	if !exists {
		// Write the canonical doc before deleting the legacy one so an interrupted
		// run leaves a recoverable duplicate, never a lost save.
		if err := h.store.Save(ctx, canonical); err != nil {
			return SavedApplication{}, err
		}
	}

	// The canonical doc is the survivor — drop the legacy duplicate.
	if err := h.store.Delete(ctx, legacy.UserID, legacy.ApplicationUID); err != nil {
		return SavedApplication{}, err
	}
	return canonical, nil
}

func dateToTime(d *platform.DateOnly) *time.Time {
	if d == nil {
		return nil
	}
	return d.TimePtr()
}

// apiErrorResponse mirrors the .NET ApiErrorResponse: { error, message:null }.
type apiErrorResponse struct {
	Error   string  `json:"error"`
	Message *string `json:"message"`
}

func (h *handler) writeJSON(w http.ResponseWriter, r *http.Request, v any) {
	body, err := httputil.EncodeJSON(v)
	if err != nil {
		h.serverError(w, r, "encode response", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(body); err != nil {
		h.logger.ErrorContext(r.Context(), "write response", "error", err)
	}
}

func (h *handler) writeError(w http.ResponseWriter, r *http.Request, status int, message string) {
	body, err := httputil.EncodeJSON(apiErrorResponse{Error: message})
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

func (h *handler) serverError(w http.ResponseWriter, r *http.Request, op string, err error) {
	h.logger.ErrorContext(r.Context(), "saved-application request failed", "op", op, "error", err)
	w.WriteHeader(http.StatusInternalServerError)
}
