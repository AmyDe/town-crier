package platform

import "testing"

// TestLoadConfig_ShareCardsBlobURL pins that the share-card blob account URL is
// read from SHARE_CARDS_BLOB_URL (the exact name infra emits) and defaults to
// empty — empty means the cache is unwired and the og:image handler regenerates
// on demand (#738 Slice 3).
func TestLoadConfig_ShareCardsBlobURL(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		const want = "https://sttowncrierdev.blob.core.windows.net"
		t.Setenv("SHARE_CARDS_BLOB_URL", want)

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if cfg.ShareCardsBlobURL != want {
			t.Errorf("ShareCardsBlobURL = %q, want %q", cfg.ShareCardsBlobURL, want)
		}
	})

	t.Run("default empty", func(t *testing.T) {
		t.Setenv("SHARE_CARDS_BLOB_URL", "")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if cfg.ShareCardsBlobURL != "" {
			t.Errorf("ShareCardsBlobURL = %q, want empty when unset", cfg.ShareCardsBlobURL)
		}
	})
}
