package platform

import (
	"net/http"
	"time"
)

// NewServer returns an http.Server with hardened timeouts. The zero-valued
// defaults on http.Server allow slowloris attacks; never accept them.
func NewServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
	}
}
