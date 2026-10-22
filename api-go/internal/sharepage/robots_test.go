package sharepage

import (
	"net/http"
	"testing"
)

const wantRobots = "User-agent: Twitterbot\nUser-agent: facebookexternalhit\nUser-agent: LinkedInBot\nUser-agent: Slackbot\nUser-agent: Discordbot\nUser-agent: WhatsApp\nUser-agent: TelegramBot\nAllow: /\n\nUser-agent: *\nDisallow: /og/\n"

func TestRobotsRoutes_ServesCrawlerPolicy(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	RobotsRoutes(mux)

	rec := serveOG(t, mux, "/robots.txt")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=86400" {
		t.Errorf("Cache-Control = %q", got)
	}
	if got := rec.Body.String(); got != wantRobots {
		t.Errorf("body = %q, want %q", got, wantRobots)
	}
}
