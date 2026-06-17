package geocoding

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/AmyDe/town-crier/api-go/internal/httputil"
)

// geocoder is the consumer-side view the handler needs: resolve a normalised
// postcode to coordinates. The concrete *Client satisfies it structurally; the
// handler test substitutes a hand fake.
type geocoder interface {
	Geocode(ctx context.Context, postcode string) (Coordinates, bool, error)
}

type handler struct {
	geocoder geocoder
	logger   *slog.Logger
}

// Routes registers the geocode endpoint on mux. The endpoint is authed (it is
// not in the .NET AllowAnonymous set) and has no Cosmos dependency, so it is
// always wired.
func Routes(mux *http.ServeMux, g geocoder, logger *slog.Logger) {
	h := handler{geocoder: g, logger: logger}
	mux.HandleFunc("GET /v1/geocode/{postcode}", h.geocode)
}

// geocodeResult mirrors the .NET GeocodePostcodeResult record: { coordinates }.
type geocodeResult struct {
	Coordinates Coordinates `json:"coordinates"`
}

// apiErrorResponse mirrors the .NET ApiErrorResponse: { error, message:null }.
type apiErrorResponse struct {
	Error   string  `json:"error"`
	Message *string `json:"message"`
}

// geocode implements GET /v1/geocode/{postcode}, mirroring the .NET endpoint's
// three outcomes: a malformed postcode is a 400 (the .NET ArgumentException
// path), an unresolvable one a 404 (the InvalidOperationException path), and a
// transport failure a bodyless 500 (the propagated HttpRequestException, whose
// PascalCase envelope middleware.ErrorBody backfills — as in .NET).
func (h handler) geocode(w http.ResponseWriter, r *http.Request) {
	raw := r.PathValue("postcode")

	normalised, ok := normalisePostcode(raw)
	if !ok {
		// .NET throws ArgumentException(msg, nameof(raw)); its Message — which the
		// endpoint serializes — carries the " (Parameter 'raw')" suffix the
		// runtime appends when a paramName is supplied. Reproduce it byte-for-byte.
		h.writeError(r, w, http.StatusBadRequest, "'"+raw+"' is not a valid UK postcode. (Parameter 'raw')")
		return
	}

	coords, found, err := h.geocoder.Geocode(r.Context(), normalised)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "geocode postcode", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !found {
		h.writeError(r, w, http.StatusNotFound, "Postcode '"+raw+"' could not be geocoded.")
		return
	}

	h.writeJSON(r, w, geocodeResult{Coordinates: coords})
}

func (h handler) writeJSON(r *http.Request, w http.ResponseWriter, v any) {
	body, err := httputil.EncodeJSON(v)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "encode geocode response", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(body); err != nil {
		h.logger.ErrorContext(r.Context(), "write geocode response", "error", err)
	}
}

func (h handler) writeError(r *http.Request, w http.ResponseWriter, status int, message string) {
	body, err := httputil.EncodeJSON(apiErrorResponse{Error: message})
	if err != nil {
		h.logger.ErrorContext(r.Context(), "encode geocode error", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		h.logger.ErrorContext(r.Context(), "write geocode error body", "error", err)
	}
}
