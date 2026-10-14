package main

import (
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres"
)

const (
	seoStatementTimeout = 10 * time.Minute
	seoConnCheck        = 10 * time.Second
)

// poolOptions returns the pool options for a worker mode. Only seo-catalog runs
// statements long enough to need server-side limits; a cancelled run must not
// leave its query running on the shared server.
func poolOptions(mode string) []postgres.PoolOption {
	if mode == "seo-catalog" {
		return []postgres.PoolOption{postgres.WithStatementGuards(seoStatementTimeout, seoConnCheck)}
	}
	return nil
}
