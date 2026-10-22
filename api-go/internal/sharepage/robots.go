package sharepage

import (
	"io"
	"net/http"
)

const robotsTxt = `User-agent: Twitterbot
User-agent: facebookexternalhit
User-agent: LinkedInBot
User-agent: Slackbot
User-agent: Discordbot
User-agent: WhatsApp
User-agent: TelegramBot
Allow: /

User-agent: *
Disallow: /og/
`

// RobotsRoutes registers the anonymous GET /robots.txt crawler policy for the
// API and share hosts: link-preview bots may fetch share cards, generic
// crawlers are kept off /og/. Keep the pattern in lockstep with the
// anonymousPatterns entry in cmd/api/wiring.go.
func RobotsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		if _, err := io.WriteString(w, robotsTxt); err != nil { //nolint:gosec // static text/plain body
			return
		}
	})
}
