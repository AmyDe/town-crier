package middleware

import (
	"net/http"
)

// CORS applies the API's CORS policy (GH#418): origins from configuration, any
// header and any method allowed. Headers are emitted only when an Origin header
// is present and matches a configured origin; mismatched or absent origins pass
// through with no CORS headers (the request is still served — the browser, not
// the server, enforces the policy). Credentials are never allowed.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			if _, ok := allowed[origin]; !ok {
				// Origin not configured: emit no CORS headers. The request is
				// still served; the browser blocks the response client-side.
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("Access-Control-Allow-Origin", origin)
			// Vary: Origin prevents a shared cache from serving one origin's
			// CORS-decorated response to a request from a different origin.
			w.Header().Add("Vary", "Origin")

			// Preflight: an OPTIONS request carrying Access-Control-Request-Method.
			// AllowAnyMethod / AllowAnyHeader means we echo exactly what the
			// browser asked for, then answer 204 without invoking the handler.
			if reqMethod := r.Header.Get("Access-Control-Request-Method"); r.Method == http.MethodOptions && reqMethod != "" {
				w.Header().Set("Access-Control-Allow-Methods", reqMethod)
				if reqHeaders := r.Header.Get("Access-Control-Request-Headers"); reqHeaders != "" {
					w.Header().Set("Access-Control-Allow-Headers", reqHeaders)
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
