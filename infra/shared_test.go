package main

import (
	"strings"
	"testing"
)

// TestSharePageAnalyticsQueries_RetainInvariants pins the substrings in isLikelyBotQuery and
// sharePageHumanTrafficQuery (tc-kg77x / GH #1020) that the compiler cannot check but a future
// edit could easily break silently: the prod-only filter (dev and prod share one App Insights
// component), both share-page route prefixes, and the robots.txt AI-crawler allowlist tokens
// that IsLikelyBot must still treat as bots for volume-counting purposes.
func TestSharePageAnalyticsQueries_RetainInvariants(t *testing.T) {
	t.Parallel()

	t.Run("sharePageHumanTrafficQuery", func(t *testing.T) {
		t.Parallel()

		wantSubstrings := []string{
			`tostring(Properties["deployment.environment"]) == "prod"`,
			`Path startswith "/a/" or Path startswith "/og/"`,
		}
		for _, want := range wantSubstrings {
			if !strings.Contains(sharePageHumanTrafficQuery, want) {
				t.Errorf("sharePageHumanTrafficQuery missing invariant substring %q", want)
			}
		}
	})

	t.Run("isLikelyBotQuery AI-crawler tokens", func(t *testing.T) {
		t.Parallel()

		// robots.txt (web/public/robots.txt) explicitly welcomes these AI crawlers for
		// SEO/discoverability, but welcomed still means bot traffic for this volume count.
		wantTokens := []string{
			"gptbot", "oai-searchbot", "chatgpt-user", "google-extended",
			"claudebot", "claude-user", "anthropic-ai",
			"perplexitybot", "perplexity-user", "ccbot", "applebot-extended",
		}
		for _, token := range wantTokens {
			if !strings.Contains(isLikelyBotQuery, token) {
				t.Errorf("isLikelyBotQuery missing AI-crawler token %q", token)
			}
		}
	})
}
