// Command api serves the Town Crier HTTP API — the Go port of the .NET API
// (GH#418). It must stay contract-identical to the .NET implementation until
// cutover.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/authorities"
	"github.com/AmyDe/town-crier/api-go/internal/health"
	"github.com/AmyDe/town-crier/api-go/internal/legal"
	"github.com/AmyDe/town-crier/api-go/internal/middleware"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/versionconfig"
)

func main() {
	cfg, err := platform.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	logger := platform.NewLogger(os.Stdout, cfg.LogLevel)

	mux := http.NewServeMux()
	health.Routes(mux, logger)
	versionconfig.Routes(mux, logger)
	legal.Routes(mux, logger)
	authorities.Routes(mux, logger)

	srv := platform.NewServer(":"+cfg.Port, middleware.ErrorBody(logger)(mux))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("api listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("serve", "error", err)
			stop()
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", "error", err)
	}
	logger.Info("api stopped")
}
