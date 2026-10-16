package subscriptionevents

import "testing"

func TestParseWebhooks(t *testing.T) {
	t.Parallel()
	t.Run("empty and unset", func(t *testing.T) {
		t.Parallel()
		for _, raw := range []string{"", "  ", "[]"} {
			got, err := ParseWebhooks(raw)
			if err != nil || len(got) != 0 {
				t.Errorf("ParseWebhooks(%q) = %v, %v; want none", raw, got, err)
			}
		}
	})
	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		got, err := ParseWebhooks(`[{"url":"https://example.test/hook","headers":{"Authorization":"Bearer x"},"bodyField":"text"}]`)
		if err != nil || len(got) != 1 {
			t.Fatalf("got %v, %v", got, err)
		}
		if got[0].URL != "https://example.test/hook" || got[0].Headers["Authorization"] != "Bearer x" || got[0].BodyField != "text" {
			t.Errorf("parsed = %+v", got[0])
		}
	})
	t.Run("errors", func(t *testing.T) {
		t.Parallel()
		for _, raw := range []string{`not json`, `[{"url":"http://example.test/hook"}]`, `[{"url":""}]`, `{"url":"https://a.test"}`} {
			if _, err := ParseWebhooks(raw); err == nil {
				t.Errorf("ParseWebhooks(%q) succeeded, want error", raw)
			}
		}
	})
	t.Run("error does not leak the secret", func(t *testing.T) {
		t.Parallel()
		_, err := ParseWebhooks(`[{"url":"http://secret-host.test/token123"}]`)
		if err == nil || containsAny(err.Error(), "secret-host", "token123") {
			t.Errorf("error leaks config: %v", err)
		}
	})
}
