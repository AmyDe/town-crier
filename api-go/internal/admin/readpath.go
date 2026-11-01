package admin

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/httputil"
)

type readPathSwitch interface {
	State(ctx context.Context) (applications.ReadPathState, error)
	Set(ctx context.Context, patch applications.ReadPathPatch, reason string, at time.Time) (applications.ReadPathState, error)
}

type readPathHandler struct {
	sw     readPathSwitch
	now    func() time.Time
	logger *slog.Logger
}

// readPathStatus is the response of GET and PUT /v1/admin/read-path. Source is
// "default" when no row is stored; every switch is then false.
type readPathStatus struct {
	ThinSync  bool       `json:"thinSync"`
	ThinList  bool       `json:"thinList"`
	ThinMap   bool       `json:"thinMap"`
	Source    string     `json:"source"`
	Reason    string     `json:"reason"`
	UpdatedAt *time.Time `json:"updatedAt"`
}

type readPathBody struct {
	ThinSync *bool  `json:"thinSync"`
	ThinList *bool  `json:"thinList"`
	ThinMap  *bool  `json:"thinMap"`
	Reason   string `json:"reason"`
}

// ReadPathRoutes registers GET and PUT /v1/admin/read-path, the thin-table
// switches, behind the shared admin key. PUT takes any of thinSync, thinList
// and thinMap plus an optional reason, changes only the named switches, and
// returns the status read back from the database. A body naming no switch is 400.
func ReadPathRoutes(mux *http.ServeMux, adminKey string, sw readPathSwitch, now func() time.Time, logger *slog.Logger) {
	h := &readPathHandler{sw: sw, now: now, logger: logger}
	mux.HandleFunc("GET /v1/admin/read-path", requireAdminKey(adminKey, h.get))
	mux.HandleFunc("PUT /v1/admin/read-path", requireAdminKey(adminKey, h.put))
}

func (h *readPathHandler) get(w http.ResponseWriter, r *http.Request) {
	st, err := h.sw.State(r.Context())
	if err != nil {
		h.fail(w, r, "read read-path switches", err)
		return
	}
	h.respond(w, r, st)
}

func (h *readPathHandler) put(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req readPathBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || (req.ThinSync == nil && req.ThinList == nil && req.ThinMap == nil) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	before, err := h.sw.State(r.Context())
	if err != nil {
		h.fail(w, r, "read read-path switches", err)
		return
	}
	after, err := h.sw.Set(r.Context(), applications.ReadPathPatch{ThinSync: req.ThinSync, ThinList: req.ThinList, ThinMap: req.ThinMap}, req.Reason, h.now())
	if err != nil {
		h.fail(w, r, "write read-path switches", err)
		return
	}
	h.logger.InfoContext(r.Context(), "admin.read_path_switched",
		slog.Bool("thinSyncFrom", before.ThinSync), slog.Bool("thinSyncTo", after.ThinSync),
		slog.Bool("thinListFrom", before.ThinList), slog.Bool("thinListTo", after.ThinList),
		slog.Bool("thinMapFrom", before.ThinMap), slog.Bool("thinMapTo", after.ThinMap),
		slog.String("reason", req.Reason))
	h.respond(w, r, after)
}

func (h *readPathHandler) respond(w http.ResponseWriter, r *http.Request, st applications.ReadPathState) {
	body, err := httputil.EncodeJSON(readPathStatus{
		ThinSync: st.ThinSync, ThinList: st.ThinList, ThinMap: st.ThinMap,
		Source: st.Source, Reason: st.Reason, UpdatedAt: st.UpdatedAt,
	})
	if err != nil {
		h.fail(w, r, "encode read-path status", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := w.Write(body); err != nil {
		h.logger.ErrorContext(r.Context(), "write admin response", "error", err)
	}
}

func (h *readPathHandler) fail(w http.ResponseWriter, r *http.Request, op string, err error) {
	h.logger.ErrorContext(r.Context(), "admin request failed", "op", op, "error", err)
	w.WriteHeader(http.StatusInternalServerError)
}
