// Package dormant implements the dormant-account cleanup worker mode
// (WORKER_MODE=dormant-cleanup): once a day it finds accounts inactive for the
// retention window and runs the shared internal/erasure cascade for each.
package dormant

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/erasure"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// retentionMonths is the inactivity window after which an account is erased. It
// is a constant, not configuration, so the retention promise is enforced
// uniformly in code.
const retentionMonths = 12

// Finder returns the dormant-account set: every profile last active strictly
// before the cutoff. profiles.AdminStore satisfies it via Dormant.
type Finder interface {
	Dormant(ctx context.Context, cutoff time.Time) ([]*profiles.UserProfile, error)
}

// Handler runs one dormant-cleanup cycle.
type Handler struct {
	finder   Finder
	deleters erasure.Deleters
	logger   *slog.Logger
	now      func() time.Time
}

// New builds a dormant-cleanup handler.
func New(finder Finder, deleters erasure.Deleters, logger *slog.Logger, now func() time.Time) *Handler {
	return &Handler{finder: finder, deleters: deleters, logger: logger, now: now}
}

// Run executes one cleanup cycle and returns the number of accounts fully erased.
// A scan failure is an error. A failed erasure step leaves the profile intact for
// the next daily run, is logged and is not counted; a profile already deleted
// concurrently still counts.
func (h *Handler) Run(ctx context.Context) (int, error) {
	cutoff := h.now().AddDate(0, -retentionMonths, 0)
	dormant, err := h.finder.Dormant(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("find dormant accounts: %w", err)
	}

	deleted := 0
	for _, p := range dormant {
		if err := erasure.Cascade(ctx, p.UserID, h.deleters); err != nil {
			h.logger.ErrorContext(ctx, "dormant account erasure failed; will retry next cycle", "user", p.UserID, "error", err)
			continue
		}
		deleted++
	}
	h.logger.InfoContext(ctx, "dormant cleanup cycle complete", "scanned", len(dormant), "deleted", deleted)
	return deleted, nil
}
