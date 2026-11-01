package main

import (
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres"
	"github.com/AmyDe/town-crier/api-go/internal/worker"
)

const (
	seoStatementTimeout       = 10 * time.Minute
	loadStatementTimeout      = 110 * time.Minute
	reconcileStatementTimeout = 10 * time.Minute
	connCheck                 = 10 * time.Second
)

// poolOptions returns the pool options for a worker mode. Only the modes that
// run long statements get server-side limits, so a cancelled run does not leave
// its query running on the shared server. poll gets none: its trigger writes
// rely on WHEN OTHERS, which does not catch query_canceled.
func poolOptions(mode string) []postgres.PoolOption {
	switch mode {
	case "seo-catalog":
		return []postgres.PoolOption{postgres.WithStatementGuards(seoStatementTimeout, connCheck)}
	case worker.ModeApplicationLocationsLoad:
		return []postgres.PoolOption{postgres.WithStatementGuards(loadStatementTimeout, connCheck)}
	case worker.ModeApplicationLocationsReconcile, worker.ModeApplicationLocationsReconcileAll:
		return []postgres.PoolOption{postgres.WithStatementGuards(reconcileStatementTimeout, connCheck)}
	}
	return nil
}
