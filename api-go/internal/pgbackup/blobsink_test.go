package pgbackup

import (
	"errors"
	"testing"
)

func TestNewBlobSink_FailsClosedOnMissingConfig(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, url, container string }{
		{"no url", "", "pg-backups"},
		{"no container", "https://x.blob.core.windows.net", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewBlobSink(tt.url, tt.container, ""); !errors.Is(err, ErrMissingBlobConfig) {
				t.Fatalf("error = %v, want ErrMissingBlobConfig", err)
			}
		})
	}
}
