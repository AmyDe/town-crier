package savedapplications

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/auth"
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

// appUpserter writes the master planning-application record so a save always
// points at a known application (the search path no longer upserts).
type appUpserter interface {
	Upsert(ctx context.Context, a applications.PlanningApplication) error
}

type handler struct {
	store  savedStore
	apps   appUpserter
	now    func() time.Time
	logger *slog.Logger
}

// Routes registers the saved-application endpoints. PUT/DELETE use a {**uid}
// catch-all so a slash-bearing application uid is captured whole, mirroring the
// .NET {**applicationUid} route.
func Routes(mux *http.ServeMux, store savedStore, apps appUpserter, now func() time.Time, logger *slog.Logger) {
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
// user's saved applications rendered from their embedded snapshots. Rows whose
// snapshot is absent (legacy, pre-snapshot-column) are skipped here; the lazy
// backfill / re-key / dedup machinery the .NET handler runs for them is deferred
// to bead tc-wans.
func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())

	saved, err := h.store.GetByUserID(r.Context(), userID)
	if err != nil {
		h.serverError(w, r, "list saved applications", err)
		return
	}
	entries := make([]savedEntry, 0, len(saved))
	for _, s := range saved {
		if s.Application == nil {
			continue
		}
		entries = append(entries, savedEntry{
			ApplicationUID: s.ApplicationUID,
			SavedAt:        platform.DotNetTime(s.SavedAt),
			Application:    applications.ResultOf(*s.Application),
		})
	}
	h.writeJSON(w, r, entries)
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
	body, err := encodeJSON(v)
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

func encodeJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func (h *handler) serverError(w http.ResponseWriter, r *http.Request, op string, err error) {
	h.logger.ErrorContext(r.Context(), "saved-application request failed", "op", op, "error", err)
	w.WriteHeader(http.StatusInternalServerError)
}
