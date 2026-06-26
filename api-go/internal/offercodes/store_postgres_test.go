package offercodes

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// ---------------------------------------------------------------------------
// Fake querier — sequential QueryRow queue, fixed Exec outcome, Query error.
// ---------------------------------------------------------------------------

// fakeOfferCodeQuerier is a hand-written test double for the querier interface.
// QueryRow calls are served from a FIFO queue so tests can configure the exact
// sequence of row responses for multi-step methods (RedeemWithCAS).
type fakeOfferCodeQuerier struct {
	execTag    pgconn.CommandTag
	execErr    error
	queryErr   error // returned from Query()
	rowResults []pgx.Row
	rowIdx     int
}

func (f *fakeOfferCodeQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return f.execTag, f.execErr
}

func (f *fakeOfferCodeQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	return &fakeEmptyRows{}, nil
}

func (f *fakeOfferCodeQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if f.rowIdx >= len(f.rowResults) {
		return &fakeErrorRow{err: pgx.ErrNoRows}
	}
	r := f.rowResults[f.rowIdx]
	f.rowIdx++
	return r
}

// fakeErrorRow returns a pre-configured error from Scan.
type fakeErrorRow struct{ err error }

func (r *fakeErrorRow) Scan(_ ...any) error { return r.err }

// fakeCodeRow scans a single code string into the first dest (the RETURNING
// clause of the RedeemWithCAS UPDATE).
type fakeCodeRow struct{ code string }

func (r *fakeCodeRow) Scan(dest ...any) error {
	if len(dest) > 0 {
		if s, ok := dest[0].(*string); ok {
			*s = r.code
		}
	}
	return nil
}

// fakeBoolRow scans a single boolean value (the `redeemed` existence check).
type fakeBoolRow struct{ value bool }

func (r *fakeBoolRow) Scan(dest ...any) error {
	if len(dest) > 0 {
		if b, ok := dest[0].(*bool); ok {
			*b = r.value
		}
	}
	return nil
}

// fakeFullCodeRow scans all 7 columns returned by the Get query in the order
// expected by scanCode: code, tier, duration_days, created_at, redeemed,
// redeemed_by_user_id, redeemed_at.
type fakeFullCodeRow struct {
	code             string
	tier             string
	durationDays     int
	createdAt        time.Time
	redeemed         bool
	redeemedByUserID *string
	redeemedAt       *time.Time
}

func (r *fakeFullCodeRow) Scan(dest ...any) error {
	if len(dest) != 7 {
		return fmt.Errorf("fakeFullCodeRow: expected 7 scan destinations, got %d", len(dest))
	}
	*dest[0].(*string) = r.code
	*dest[1].(*string) = r.tier
	*dest[2].(*int) = r.durationDays
	*dest[3].(*time.Time) = r.createdAt
	*dest[4].(*bool) = r.redeemed
	*dest[5].(**string) = r.redeemedByUserID
	*dest[6].(**time.Time) = r.redeemedAt
	return nil
}

// fakeEmptyRows is a pgx.Rows that immediately reports no rows, used for the
// empty-result Query path (e.g. RedeemedByUserID with no matches).
type fakeEmptyRows struct{}

func (r *fakeEmptyRows) Next() bool                                   { return false }
func (r *fakeEmptyRows) Scan(_ ...any) error                          { return nil }
func (r *fakeEmptyRows) Values() ([]any, error)                       { return nil, nil }
func (r *fakeEmptyRows) Close()                                       {}
func (r *fakeEmptyRows) Err() error                                   { return nil }
func (r *fakeEmptyRows) RawValues() [][]byte                          { return nil }
func (r *fakeEmptyRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeEmptyRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeEmptyRows) Conn() *pgx.Conn                              { return nil }

// ---------------------------------------------------------------------------
// Compile-time parity: PostgresStore must satisfy the same consumer interfaces
// as CosmosStore. Add here so a divergence is a compile error.
// ---------------------------------------------------------------------------

var (
	_ codeStore = (*PostgresStore)(nil)
	_ codeStore = (*CosmosStore)(nil)
)

// ---------------------------------------------------------------------------
// Get
// ---------------------------------------------------------------------------

// TestPostgresStore_Get_Miss confirms that a pgx.ErrNoRows from the point-read
// query surfaces as ErrNotFound (not a raw pgx error).
func TestPostgresStore_Get_Miss(t *testing.T) {
	t.Parallel()
	store := NewPostgresStore(&fakeOfferCodeQuerier{
		rowResults: []pgx.Row{&fakeErrorRow{err: pgx.ErrNoRows}},
	})
	_, err := store.Get(context.Background(), "ABCDEFGHJKMN")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get miss: got %v, want ErrNotFound", err)
	}
}

// TestPostgresStore_Get_DatabaseError confirms that a non-ErrNoRows query error
// is wrapped and returned.
func TestPostgresStore_Get_DatabaseError(t *testing.T) {
	t.Parallel()
	boom := errors.New("connection refused")
	store := NewPostgresStore(&fakeOfferCodeQuerier{
		rowResults: []pgx.Row{&fakeErrorRow{err: boom}},
	})
	_, err := store.Get(context.Background(), "ABCDEFGHJKMN")
	if !errors.Is(err, boom) {
		t.Fatalf("Get db error: got %v, want wrapped %v", err, boom)
	}
}

// TestPostgresStore_Get_Hit confirms a successful full-row scan returns a
// correctly hydrated OfferCode.
func TestPostgresStore_Get_Hit(t *testing.T) {
	t.Parallel()
	created := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	store := NewPostgresStore(&fakeOfferCodeQuerier{
		rowResults: []pgx.Row{&fakeFullCodeRow{
			code: "ABCDEFGHJKMN", tier: "Pro", durationDays: 30,
			createdAt: created, redeemed: false,
		}},
	})
	got, err := store.Get(context.Background(), "ABCDEFGHJKMN")
	if err != nil {
		t.Fatalf("Get hit: %v", err)
	}
	if got.Code != "ABCDEFGHJKMN" {
		t.Errorf("Code: got %q, want %q", got.Code, "ABCDEFGHJKMN")
	}
	if got.Tier != profiles.TierPro {
		t.Errorf("Tier: got %v, want TierPro", got.Tier)
	}
	if got.DurationDays != 30 {
		t.Errorf("DurationDays: got %d, want 30", got.DurationDays)
	}
	if !got.CreatedAt.Equal(created) {
		t.Errorf("CreatedAt: got %v, want %v", got.CreatedAt, created)
	}
	if got.IsRedeemed() {
		t.Error("freshly stored code should not be redeemed")
	}
}

// ---------------------------------------------------------------------------
// Save
// ---------------------------------------------------------------------------

// TestPostgresStore_Save_DatabaseError confirms that an Exec error is wrapped
// and returned.
func TestPostgresStore_Save_DatabaseError(t *testing.T) {
	t.Parallel()
	boom := errors.New("deadlock detected")
	store := NewPostgresStore(&fakeOfferCodeQuerier{execErr: boom})
	created := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	code, _ := NewOfferCode("ABCDEFGHJKMN", profiles.TierPro, 30, created)
	if err := store.Save(context.Background(), code); !errors.Is(err, boom) {
		t.Fatalf("Save db error: got %v, want wrapped %v", err, boom)
	}
}

// ---------------------------------------------------------------------------
// RedeemWithCAS
// ---------------------------------------------------------------------------

// TestPostgresStore_RedeemWithCAS_Success confirms that when the atomic UPDATE
// WHERE redeemed=false matches and returns a row, RedeemWithCAS returns nil.
func TestPostgresStore_RedeemWithCAS_Success(t *testing.T) {
	t.Parallel()
	store := NewPostgresStore(&fakeOfferCodeQuerier{
		rowResults: []pgx.Row{
			// First QueryRow: UPDATE ... WHERE redeemed=false RETURNING code
			&fakeCodeRow{code: "ABCDEFGHJKMN"},
		},
	})
	err := store.RedeemWithCAS(context.Background(), "ABCDEFGHJKMN", "auth0|u1", time.Now().UTC())
	if err != nil {
		t.Fatalf("RedeemWithCAS success: got %v, want nil", err)
	}
}

// TestPostgresStore_RedeemWithCAS_AlreadyRedeemed confirms that when the UPDATE
// matches no rows (ErrNoRows) and the follow-up existence check finds the code
// with redeemed=true, ErrAlreadyRedeemed is returned.
func TestPostgresStore_RedeemWithCAS_AlreadyRedeemed(t *testing.T) {
	t.Parallel()
	store := NewPostgresStore(&fakeOfferCodeQuerier{
		rowResults: []pgx.Row{
			&fakeErrorRow{err: pgx.ErrNoRows}, // UPDATE matched nothing
			&fakeBoolRow{value: true},         // code exists, redeemed=true
		},
	})
	err := store.RedeemWithCAS(context.Background(), "ABCDEFGHJKMN", "auth0|u1", time.Now().UTC())
	if !errors.Is(err, ErrAlreadyRedeemed) {
		t.Fatalf("RedeemWithCAS already redeemed: got %v, want ErrAlreadyRedeemed", err)
	}
}

// TestPostgresStore_RedeemWithCAS_NotFound confirms that when the UPDATE matches
// no rows and the follow-up existence check also finds no row, ErrNotFound is
// returned.
func TestPostgresStore_RedeemWithCAS_NotFound(t *testing.T) {
	t.Parallel()
	store := NewPostgresStore(&fakeOfferCodeQuerier{
		rowResults: []pgx.Row{
			&fakeErrorRow{err: pgx.ErrNoRows}, // UPDATE matched nothing
			&fakeErrorRow{err: pgx.ErrNoRows}, // code doesn't exist either
		},
	})
	err := store.RedeemWithCAS(context.Background(), "ABCDEFGHJKMN", "auth0|u1", time.Now().UTC())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("RedeemWithCAS not found: got %v, want ErrNotFound", err)
	}
}

// ---------------------------------------------------------------------------
// RedeemedByUserID
// ---------------------------------------------------------------------------

// TestPostgresStore_RedeemedByUserID_QueryError confirms that a Query error is
// wrapped and returned.
func TestPostgresStore_RedeemedByUserID_QueryError(t *testing.T) {
	t.Parallel()
	boom := errors.New("db down")
	store := NewPostgresStore(&fakeOfferCodeQuerier{queryErr: boom})
	_, err := store.RedeemedByUserID(context.Background(), "auth0|u1")
	if !errors.Is(err, boom) {
		t.Fatalf("RedeemedByUserID query error: got %v, want wrapped %v", err, boom)
	}
}

// TestPostgresStore_RedeemedByUserID_EmptyReturnsNonNilSlice confirms that a
// user who has redeemed nothing gets an empty, non-nil slice (not nil), as the
// GDPR export relies on.
func TestPostgresStore_RedeemedByUserID_EmptyReturnsNonNilSlice(t *testing.T) {
	t.Parallel()
	store := NewPostgresStore(&fakeOfferCodeQuerier{}) // Query returns emptyRows
	got, err := store.RedeemedByUserID(context.Background(), "auth0|never")
	if err != nil {
		t.Fatalf("RedeemedByUserID empty: %v", err)
	}
	if got == nil {
		t.Error("must return non-nil empty slice, not nil")
	}
	if len(got) != 0 {
		t.Errorf("count: got %d, want 0", len(got))
	}
}

// ---------------------------------------------------------------------------
// AnonymiseRedemptionsByUserID
// ---------------------------------------------------------------------------

// TestPostgresStore_AnonymiseRedemptionsByUserID_ExecError confirms that an
// Exec error (the UPDATE) is wrapped and returned.
func TestPostgresStore_AnonymiseRedemptionsByUserID_ExecError(t *testing.T) {
	t.Parallel()
	boom := errors.New("db timeout")
	store := NewPostgresStore(&fakeOfferCodeQuerier{execErr: boom})
	if err := store.AnonymiseRedemptionsByUserID(context.Background(), "auth0|u1"); !errors.Is(err, boom) {
		t.Fatalf("AnonymiseRedemptionsByUserID exec error: got %v, want wrapped %v", err, boom)
	}
}

// ---------------------------------------------------------------------------
// Legacy-coalesce hydration (DecodeDocument / toDomain)
// ---------------------------------------------------------------------------

// TestDecodeDocument_LegacyCoalesceHydration verifies that a Cosmos document
// with redeemed=false but a non-nil redeemedByUserId (data written before the
// redeemed boolean column existed) decodes with IsRedeemed()=true, so the code
// can never be re-redeemed.
func TestDecodeDocument_LegacyCoalesceHydration(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"id":"ABCDEFGHJKMN","code":"ABCDEFGHJKMN","tier":"Pro","durationDays":30,` +
		`"createdAt":"2026-01-01T00:00:00+00:00","redeemed":false,"redeemedByUserId":"auth0|legacy"}`)
	code, err := DecodeDocument(raw)
	if err != nil {
		t.Fatalf("DecodeDocument: %v", err)
	}
	if !code.IsRedeemed() {
		t.Error("legacy code with non-nil RedeemedByUserID must be considered redeemed")
	}
	if code.RedeemedByUserID == nil || *code.RedeemedByUserID != "auth0|legacy" {
		t.Errorf("RedeemedByUserID: got %v, want auth0|legacy", code.RedeemedByUserID)
	}
}

// TestPostgresStore_Get_LegacyCoalesceHydration verifies the Postgres scanCode
// path: a row with redeemed=false but a non-nil redeemed_by_user_id returns
// IsRedeemed()=true (matching the toDomain legacy coalesce rule).
func TestPostgresStore_Get_LegacyCoalesceHydration(t *testing.T) {
	t.Parallel()
	legacyUserID := "auth0|legacy"
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	store := NewPostgresStore(&fakeOfferCodeQuerier{
		rowResults: []pgx.Row{&fakeFullCodeRow{
			code: "ABCDEFGHJKMN", tier: "Pro", durationDays: 30,
			createdAt:        created,
			redeemed:         false, // legacy: tombstone not yet written
			redeemedByUserID: &legacyUserID,
		}},
	})
	got, err := store.Get(context.Background(), "ABCDEFGHJKMN")
	if err != nil {
		t.Fatalf("Get legacy: %v", err)
	}
	if !got.IsRedeemed() {
		t.Error("legacy code with non-nil RedeemedByUserID must be considered redeemed")
	}
}
