package middleware

import "net/http"

// StripForwardedFor removes the X-Forwarded-For header from every inbound
// request before anything downstream sees it.
//
// The point is privacy, not hygiene. otelhttp feeds the header into the
// OpenTelemetry client.address span attribute, which was putting real end-user
// IP addresses into Application Insights AppRequests, retained for 30 days.
// That contradicts the project's stated position that we do not log client IPs
// (privacy.json: "we do not track"). Azure's own built-in ClientIP column is
// already masked to 0.0.0.0, so this custom attribute was the only leak.
//
// This demotes the attribute rather than deleting it. otelhttp resolves the
// client IP as options -> X-Forwarded-For -> TCP peer, so with the header gone
// client.address falls back to the peer, which behind Cloudflare and Container
// Apps ingress is the ingress address (100.100.0.x) — infrastructure, identical
// for every caller, not a person. Forcing the attribute to disappear outright
// would mean blanking r.RemoteAddr too, and that is not worth it: internal/
// clientip parses RemoteAddr to decide whether CF-Connecting-IP can be trusted,
// and per anonratelimit.go an unparseable RemoteAddr collapses every anonymous
// caller onto one shared rate-limit bucket. Cosmetic gain, availability risk.
//
// Nothing in this codebase reads X-Forwarded-For: internal/clientip documents
// that it is deliberately never read (it is trivially spoofable), and takes the
// caller's IP from CF-Connecting-IP validated against the published Cloudflare
// ranges instead. That header is untouched here, so the anonymous rate limiter
// keeps working.
//
// This must wrap OUTSIDE otelhttp.NewHandler so the header is gone before the
// span is started; see cmd/api/wiring.go.
func StripForwardedFor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Del("X-Forwarded-For")
		next.ServeHTTP(w, r)
	})
}
