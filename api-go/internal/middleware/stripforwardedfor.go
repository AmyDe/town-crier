package middleware

import "net/http"

// StripForwardedFor removes X-Forwarded-For from every inbound request so
// otelhttp cannot record end-user IPs as the client.address span attribute in
// Application Insights; client.address falls back to the ingress peer address.
// Client-IP resolution uses CF-Connecting-IP (internal/clientip), never this
// header. It must wrap OUTSIDE otelhttp.NewHandler so the header is gone before
// the span starts.
func StripForwardedFor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Del("X-Forwarded-For")
		next.ServeHTTP(w, r)
	})
}
