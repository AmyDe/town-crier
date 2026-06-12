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

	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

func main() {
	cfg, err := platform.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	logger := platform.NewLogger(os.Stdout, cfg.LogLevel)

	validator, err := auth.NewAuth0Validator(cfg.Auth0Domain, cfg.Auth0Audience, logger)
	if err != nil {
		log.Fatal(err)
	}

	cosmos, err := platform.NewCosmosContainer(cfg, logger)
	if err != nil {
		log.Fatal(err)
	}
	var store *profiles.CosmosStore
	if cosmos != nil {
		store = profiles.NewCosmosStore(cosmos)
	}

	// Real M2M client only when fully configured; otherwise the no-op fallback,
	// matching .NET's conditional IAuth0ManagementClient registration.
	var manager profiles.Auth0Manager = profiles.NoOpAuth0Client{}
	if cfg.Auth0M2MConfigured() {
		manager = profiles.NewAuth0Client(
			&http.Client{Timeout: 30 * time.Second},
			"https://"+cfg.Auth0Domain,
			cfg.Auth0M2MClientID,
			cfg.Auth0M2MClientSecret,
		)
	}

	srv := platform.NewServer(":"+cfg.Port, newRouter(validator, cfg.CorsAllowedOrigins, store, manager, cfg.ProDomains, logger))

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
