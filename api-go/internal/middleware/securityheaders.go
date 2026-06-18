package middleware

import "net/http"

// SecurityHeaders sets the X-Content-Type-Options header on every response,
// before the next handler writes, so it applies to both success and error
// responses. Only nosniff is set: CSP, X-Frame-Options, and HSTS are
// out-of-scope for this cookieless JSON API behind TLS ingress (GH#521).
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}
