package designations

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
)

// provider is the consumer-side view the handler needs: resolve the designation
// context for a point. The concrete *Client satisfies it structurally; the
// handler test substitutes a hand fake.
type provider interface {
	Get(ctx context.Context, latitude, longitude float64) (Context, error)
}

type handler struct {
	provider provider
	logger   *slog.Logger
}

// Routes registers the designations endpoint on mux. The endpoint is authed (it
// is not in the .NET AllowAnonymous set) and has no Cosmos dependency, so it is
// always wired.
func Routes(mux *http.ServeMux, p provider, logger *slog.Logger) {
	h := handler{provider: p, logger: logger}
	mux.HandleFunc("GET /v1/designations", h.designations)
}

// designationsResult mirrors the .NET GetDesignationContextResult record; field
// order matches so the wire bytes are identical.
type designationsResult struct {
	IsWithinConservationArea        bool    `json:"isWithinConservationArea"`
	ConservationAreaName            *string `json:"conservationAreaName"`
	IsWithinListedBuildingCurtilage bool    `json:"isWithinListedBuildingCurtilage"`
	ListedBuildingGrade             *string `json:"listedBuildingGrade"`
	IsWithinArticle4Area            bool    `json:"isWithinArticle4Area"`
}

// designations implements GET /v1/designations?latitude=&longitude=. Missing or
// unparseable coordinates are a bodyless 400 (mirroring .NET's value-type query
// binding failure, whose PascalCase envelope middleware.ErrorBody backfills). A
// provider failure degrades to the empty context, mirroring the .NET handler's
// catch of HttpRequestException — the endpoint always answers 200 once the
// coordinates parse.
func (h handler) designations(w http.ResponseWriter, r *http.Request) {
	latitude, latErr := strconv.ParseFloat(r.URL.Query().Get("latitude"), 64)
	longitude, lngErr := strconv.ParseFloat(r.URL.Query().Get("longitude"), 64)
	if latErr != nil || lngErr != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	designation, err := h.provider.Get(r.Context(), latitude, longitude)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "resolve designations", "error", err)
		designation = Context{}
	}

	// designationsResult mirrors Context field-for-field, differing only in JSON
	// tags (ignored in a struct conversion), so the result is just the context in
	// wire dress.
	h.writeJSON(r, w, designationsResult(designation))
}

func (h handler) writeJSON(r *http.Request, w http.ResponseWriter, v any) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		h.logger.ErrorContext(r.Context(), "encode designations response", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := w.Write(bytes.TrimRight(buf.Bytes(), "\n")); err != nil {
		h.logger.ErrorContext(r.Context(), "write designations response", "error", err)
	}
}
