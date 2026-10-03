package admin

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/httputil"
	"github.com/AmyDe/town-crier/api-go/internal/polling"
)

type pollingSwitch interface {
	State(ctx context.Context) (polling.SwitchState, error)
	Set(ctx context.Context, enabled bool, reason string, at time.Time) (polling.SwitchState, error)
}

type planItCallReader interface {
	Latest(ctx context.Context) (*polling.PlanItCall, error)
	CountBetween(ctx context.Context, from, to time.Time) (int, error)
}

type pollingHandler struct {
	sw     pollingSwitch
	calls  planItCallReader
	now    func() time.Time
	logger *slog.Logger
}

// pollingStatus is the response of GET and PUT /v1/admin/polling. Source is
// "default" when no switch is stored and the environment default applies.
// LastCallAt and LastCallStatus describe the latest planit_call row;
// LastCallStatus is null for a timeout and 0 for an aborted call. CallsToday
// counts the current 18:00-to-18:00 Europe/London budget day.
type pollingStatus struct {
	Enabled        bool       `json:"enabled"`
	Source         string     `json:"source"`
	Reason         string     `json:"reason"`
	UpdatedAt      *time.Time `json:"updatedAt"`
	LastCallAt     *time.Time `json:"lastCallAt"`
	LastCallStatus *int       `json:"lastCallStatus"`
	CallsToday     int        `json:"callsToday"`
}

type pollingRequest struct {
	Enabled *bool  `json:"enabled"`
	Reason  string `json:"reason"`
}

// PollingRoutes registers GET and PUT /v1/admin/polling, the switch that lets
// this environment poll PlanIt, behind the shared admin key. PUT takes
// {"enabled": bool, "reason": string}; a missing enabled is 400. Both return
// the status read back from the database.
func PollingRoutes(mux *http.ServeMux, adminKey string, sw pollingSwitch, calls planItCallReader, now func() time.Time, logger *slog.Logger) {
	h := &pollingHandler{sw: sw, calls: calls, now: now, logger: logger}
	mux.HandleFunc("GET /v1/admin/polling", requireAdminKey(adminKey, h.get))
	mux.HandleFunc("PUT /v1/admin/polling", requireAdminKey(adminKey, h.put))
}

func (h *pollingHandler) get(w http.ResponseWriter, r *http.Request) {
	st, err := h.sw.State(r.Context())
	if err != nil {
		h.fail(w, r, "read polling switch", err)
		return
	}
	h.respond(w, r, st)
}

func (h *pollingHandler) put(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req pollingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Enabled == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	before, err := h.sw.State(r.Context())
	if err != nil {
		h.fail(w, r, "read polling switch", err)
		return
	}
	after, err := h.sw.Set(r.Context(), *req.Enabled, req.Reason, h.now())
	if err != nil {
		h.fail(w, r, "write polling switch", err)
		return
	}
	h.logger.InfoContext(r.Context(), "admin.polling_switched",
		slog.Bool("from", before.Enabled), slog.Bool("to", after.Enabled), slog.String("reason", req.Reason))
	h.respond(w, r, after)
}

func (h *pollingHandler) respond(w http.ResponseWriter, r *http.Request, st polling.SwitchState) {
	out := pollingStatus{Enabled: st.Enabled, Source: st.Source, Reason: st.Reason, UpdatedAt: st.UpdatedAt}
	latest, err := h.calls.Latest(r.Context())
	if err != nil {
		h.fail(w, r, "read latest planit call", err)
		return
	}
	if latest != nil {
		at := latest.At
		out.LastCallAt, out.LastCallStatus = &at, latest.Status
	}
	from, to := polling.BudgetDay(h.now())
	if out.CallsToday, err = h.calls.CountBetween(r.Context(), from, to); err != nil {
		h.fail(w, r, "count planit calls", err)
		return
	}
	body, err := httputil.EncodeJSON(out)
	if err != nil {
		h.fail(w, r, "encode polling status", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := w.Write(body); err != nil {
		h.logger.ErrorContext(r.Context(), "write admin response", "error", err)
	}
}

func (h *pollingHandler) fail(w http.ResponseWriter, r *http.Request, op string, err error) {
	h.logger.ErrorContext(r.Context(), "admin request failed", "op", op, "error", err)
	w.WriteHeader(http.StatusInternalServerError)
}
