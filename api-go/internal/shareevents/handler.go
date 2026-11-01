// Package shareevents counts completed shares as daily totals by destination.
// A request carries only the destination: no link code, application or user.
package shareevents

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

const (
	maxBodyBytes     = 1 << 10
	otherDestination = "other"
)

var destinations = map[string]struct{}{
	"messages":       {},
	"mail":           {},
	"whatsapp":       {},
	"facebook":       {},
	"copy":           {},
	otherDestination: {},
}

type handler struct {
	store  Store
	now    func() time.Time
	logger *slog.Logger
}

// Routes registers the anonymous POST /v1/share-events endpoint. It accepts
// {"destination":"<value>"}, counts any unknown destination as "other", and
// returns 204. A body that is malformed or over 1 KiB returns 400. Keep the
// pattern in lockstep with the anonymousPatterns entry in cmd/api/wiring.go.
func Routes(mux *http.ServeMux, store Store, now func() time.Time, logger *slog.Logger) {
	h := &handler{store: store, now: now, logger: logger}
	mux.HandleFunc("POST /v1/share-events", h.record)
}

func (h *handler) record(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var body struct {
		Destination string `json:"destination"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	destination := body.Destination
	if _, known := destinations[destination]; !known {
		destination = otherDestination
	}

	day := h.now().UTC().Truncate(24 * time.Hour)
	if err := h.store.Increment(r.Context(), day, destination); err != nil {
		platform.RespondRequestError(w, r, h.logger, "share event write failed", err, "op", "increment share total")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
