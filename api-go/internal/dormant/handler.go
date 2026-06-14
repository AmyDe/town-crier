// Package dormant holds the dormant-account cleanup worker mode
// (WORKER_MODE=dormant-cleanup): once a day it finds user accounts inactive for
// the retention window and runs the full GDPR erasure cascade for each — deleting
// the user's notifications, watch zones, saved applications, device
// registrations, and notification-state watermark from Cosmos, then the profile,
// then the Auth0 user via the Management (M2M) API.
//
// The cascade itself lives in internal/erasure so the dormant worker and the
// DELETE /v1/me HTTP handler share one ordered erasure (bead tc-gf0g); this
// package only scans for dormant accounts and drives the cascade per account.
//
// It ports the .NET DormantAccountCleanupCommandHandler + DeleteUserProfile
// CommandHandler (epic tc-wad3, bead tc-dwcq) following idiomatic Go:
// consumer-side interfaces declared here, concrete stores injected from main(),
// and hand-written test fakes. The 12-month retention window is a code constant
// (not config) so the privacy policy's "12 months of inactivity" promise is
// enforced uniformly (UK GDPR Art. 5(1)(e), ADR 0023).
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
// uniformly in code, mirroring .NET's DormantAccountCleanupCommandHandler.
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

// New builds a dormant-cleanup handler. now is injected so tests pin the cutoff;
// production passes time.Now. The deleters carry the per-container erasure steps
// and the profile-absent predicate that lets the cascade tolerate a
// concurrently-deleted profile.
func New(finder Finder, deleters erasure.Deleters, logger *slog.Logger, now func() time.Time) *Handler {
	return &Handler{finder: finder, deleters: deleters, logger: logger, now: now}
}

// Run executes one cleanup cycle and returns the number of accounts fully erased.
// It scans for dormant accounts, then runs the erasure cascade for each. A child
// step that fails leaves the profile intact (so the next daily run retries) and
// the account is not counted; a profile that has already been deleted by a
// concurrent caller is tolerated and still counted (its end state is achieved).
// A scan failure is fatal to the cycle; per-account failures are logged and the
// run continues, mirroring .NET's per-account try/catch.
func (h *Handler) Run(ctx context.Context) (int, error) {
	cutoff := h.now().AddDate(0, -retentionMonths, 0)
	dormant, err := h.finder.Dormant(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("find dormant accounts: %w", err)
	}

	deleted := 0
	for _, p := range dormant {
		if err := h.erase(ctx, p.UserID); err != nil {
			h.logger.ErrorContext(ctx, "dormant account erasure failed; will retry next cycle", "user", p.UserID, "error", err)
			continue
		}
		deleted++
	}
	h.logger.InfoContext(ctx, "dormant cleanup cycle complete", "scanned", len(dormant), "deleted", deleted)
	return deleted, nil
}

// erase runs the shared GDPR Art. 17 erasure cascade for one user. The ordering
// and profile-absent tolerance live in internal/erasure; the profile-absent
// predicate (errors.Is(err, profiles.ErrNotFound)) is supplied by main() when it
// builds the deleters.
func (h *Handler) erase(ctx context.Context, userID string) error {
	return erasure.Cascade(ctx, userID, h.deleters)
}
