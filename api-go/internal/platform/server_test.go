package platform

import (
	"net/http"
	"testing"
)

// The hardened profile is the point of NewServer: a zero timeout reintroduces
// slowloris exposure, so regressions must fail loudly.
func TestNewServer_HardenedTimeouts(t *testing.T) {
	t.Parallel()

	srv := NewServer(":8080", http.NewServeMux())

	if srv.ReadHeaderTimeout <= 0 {
		t.Error("ReadHeaderTimeout must be positive")
	}
	if srv.ReadTimeout <= 0 {
		t.Error("ReadTimeout must be positive")
	}
	if srv.WriteTimeout <= 0 {
		t.Error("WriteTimeout must be positive")
	}
	if srv.IdleTimeout <= 0 {
		t.Error("IdleTimeout must be positive")
	}
	if srv.MaxHeaderBytes <= 0 {
		t.Error("MaxHeaderBytes must be positive")
	}
	if srv.Addr != ":8080" {
		t.Errorf("Addr: got %q, want %q", srv.Addr, ":8080")
	}
}
