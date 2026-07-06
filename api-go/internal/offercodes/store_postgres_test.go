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
// Fake querier — sequential QueryRow queue, fixed Exec outcome, Query error,
// plus Begin() returning a configurable fake transaction.
// ---------------------------------------------------------------------------

// fakeOfferCodeQuerier is a hand-written test double for the pool interface.
// QueryRow calls are served from a FIFO queue so tests can configure the exact
// sequence of row responses for multi-step methods; Begin returns the
// configured fakeTx (or beginErr).
type fakeOfferCodeQuerier struct {
	execTag    pgconn.CommandTag
	execErr    error
	queryErr   error    // returned from Query()
	queryRows  pgx.Rows // returned from Query() when non-nil (else fakeEmptyRows)
	rowResults []pgx.Row
	rowIdx     int
	beginErr   error
	tx         *fakeTx

	gotQuery string
	gotArgs  []any
}

func (f *fakeOfferCodeQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return f.execTag, f.execErr
}

func (f *fakeOfferCodeQuerier) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	f.gotQuery = sql
	f.gotArgs = args
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	if f.queryRows != nil {
		return f.queryRows, nil
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

func (f *fakeOfferCodeQuerier) Begin(_ context.Context) (pgx.Tx, error) {
	if f.beginErr != nil {
		return nil, f.beginErr
	}
	if f.tx == nil {
		f.tx = &fakeTx{}
	}
	return f.tx, nil
}

// fakeTx is a hand-written pgx.Tx test double for the transactional
// RedeemWithCAS / AnonymiseRedemptionsByUserID paths. Exec responses come from
// a FIFO error queue (the offer_code_redemptions INSERT, or either anonymise
// UPDATE); QueryRow responses from a FIFO row queue (the redemption_count
// UPDATE...RETURNING and the not-found/fully-consumed existence check).
// Commit/Rollback are recorded so tests can assert which one fired.
type fakeTx struct {
	execResults []error
	execIdx     int
	rowResults  []pgx.Row
	rowIdx      int
	commitErr   error
	committed   bool
	rolledBack  bool
}

func (f *fakeTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	if f.execIdx >= len(f.execResults) {
		return pgconn.CommandTag{}, nil
	}
	err := f.execResults[f.execIdx]
	f.execIdx++
	return pgconn.CommandTag{}, err
}

func (f *fakeTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return &fakeEmptyRows{}, nil
}

func (f *fakeTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if f.rowIdx >= len(f.rowResults) {
		return &fakeErrorRow{err: pgx.ErrNoRows}
	}
	r := f.rowResults[f.rowIdx]
	f.rowIdx++
	return r
}

func (f *fakeTx) Commit(_ context.Context) error {
	f.committed = true
	return f.commitErr
}

func (f *fakeTx) Rollback(_ context.Context) error {
	f.rolledBack = true
	return nil
}

// The remaining methods satisfy the full pgx.Tx interface; none are exercised
// by these tests.
func (f *fakeTx) Begin(_ context.Context) (pgx.Tx, error) { return f, nil }
func (f *fakeTx) CopyFrom(_ context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (f *fakeTx) SendBatch(_ context.Context, _ *pgx.Batch) pgx.BatchResults { return nil }
func (f *fakeTx) LargeObjects() pgx.LargeObjects                             { return pgx.LargeObjects{} }
func (f *fakeTx) Prepare(_ context.Context, _, _ string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (f *fakeTx) Conn() *pgx.Conn { return nil }

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

// fakeIntRow scans a single int value (the offer-code existence check).
type fakeIntRow struct{ value int }

func (r *fakeIntRow) Scan(dest ...any) error {
	if len(dest) > 0 {
		if v, ok := dest[0].(*int); ok {
			*v = r.value
		}
	}
	return nil
}

// fakeFullCodeRow scans all 7 columns returned by the Get query in the order
// expected by scanCode: code, tier, duration_days, created_at, label,
// max_redemptions, redemption_count.
type fakeFullCodeRow struct {
	code            string
	tier            string
	durationDays    int
	createdAt       time.Time
	label           *string
	maxRedemptions  int
	redemptionCount int
}

func (r *fakeFullCodeRow) Scan(dest ...any) error {
	if len(dest) != 7 {
		return fmt.Errorf("fakeFullCodeRow: expected 7 scan destinations, got %d", len(dest))
	}
	*dest[0].(*string) = r.code
	*dest[1].(*string) = r.tier
	*dest[2].(*int) = r.durationDays
	*dest[3].(*time.Time) = r.createdAt
	*dest[4].(**string) = r.label
	*dest[5].(*int) = r.maxRedemptions
	*dest[6].(*int) = r.redemptionCount
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
// Compile-time check: PostgresStore satisfies the handler's consumer interface,
// so a divergence is a compile error.
// ---------------------------------------------------------------------------

var _ codeStore = (*PostgresStore)(nil)

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
// correctly hydrated OfferCode, including the multi-redemption fields.
func TestPostgresStore_Get_Hit(t *testing.T) {
	t.Parallel()
	created := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	label := "creator-campaign"
	store := NewPostgresStore(&fakeOfferCodeQuerier{
		rowResults: []pgx.Row{&fakeFullCodeRow{
			code: "ABCDEFGHJKMN", tier: "Pro", durationDays: 30,
			createdAt: created, label: &label, maxRedemptions: 3, redemptionCount: 1,
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
	if got.Label != "creator-campaign" {
		t.Errorf("Label: got %q, want %q", got.Label, "creator-campaign")
	}
	if got.MaxRedemptions != 3 || got.RedemptionCount != 1 {
		t.Errorf("MaxRedemptions/RedemptionCount: got %d/%d, want 3/1", got.MaxRedemptions, got.RedemptionCount)
	}
	if got.IsFullyRedeemed() {
		t.Error("a code with free slots left should not be fully redeemed")
	}
}

// TestPostgresStore_Get_NullLabelCoalescesToEmpty confirms a NULL label column
// (a row that predates the label feature) hydrates as an empty string, not a
// nil-pointer panic.
func TestPostgresStore_Get_NullLabelCoalescesToEmpty(t *testing.T) {
	t.Parallel()
	store := NewPostgresStore(&fakeOfferCodeQuerier{
		rowResults: []pgx.Row{&fakeFullCodeRow{
			code: "ABCDEFGHJKMN", tier: "Pro", durationDays: 30,
			createdAt: time.Now(), label: nil, maxRedemptions: 1, redemptionCount: 0,
		}},
	})
	got, err := store.Get(context.Background(), "ABCDEFGHJKMN")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Label != "" {
		t.Errorf("Label: got %q, want empty string for a NULL column", got.Label)
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
	code, _ := NewOfferCode("ABCDEFGHJKMN", profiles.TierPro, 30, "campaign", 1, created)
	if err := store.Save(context.Background(), code); !errors.Is(err, boom) {
		t.Fatalf("Save db error: got %v, want wrapped %v", err, boom)
	}
}

// ---------------------------------------------------------------------------
// RedeemWithCAS
// ---------------------------------------------------------------------------

// TestPostgresStore_RedeemWithCAS_Success confirms that when the insert
// succeeds and the atomic UPDATE matches and returns a row, RedeemWithCAS
// returns nil and commits the transaction.
func TestPostgresStore_RedeemWithCAS_Success(t *testing.T) {
	t.Parallel()
	tx := &fakeTx{
		execResults: []error{nil}, // INSERT into offer_code_redemptions succeeds
		rowResults:  []pgx.Row{&fakeCodeRow{code: "ABCDEFGHJKMN"}},
	}
	store := NewPostgresStore(&fakeOfferCodeQuerier{tx: tx})
	err := store.RedeemWithCAS(context.Background(), "ABCDEFGHJKMN", "auth0|u1", time.Now().UTC())
	if err != nil {
		t.Fatalf("RedeemWithCAS success: got %v, want nil", err)
	}
	if !tx.committed {
		t.Error("successful redemption must commit the transaction")
	}
	// Note: the deferred tx.Rollback(ctx) still fires after a successful
	// Commit — that's the standard pgx pattern (`defer tx.Rollback(ctx)` right
	// after Begin) and is a documented no-op post-commit in real pgx, so
	// rolledBack being true here is expected and not asserted against.
}

// TestPostgresStore_RedeemWithCAS_AlreadyRedeemedByUser confirms that a unique
// violation on the offer_code_redemptions insert (the caller has already
// redeemed this code) surfaces as ErrAlreadyRedeemedByUser and rolls back.
func TestPostgresStore_RedeemWithCAS_AlreadyRedeemedByUser(t *testing.T) {
	t.Parallel()
	tx := &fakeTx{
		execResults: []error{&pgconn.PgError{Code: pgUniqueViolationCode}},
	}
	store := NewPostgresStore(&fakeOfferCodeQuerier{tx: tx})
	err := store.RedeemWithCAS(context.Background(), "ABCDEFGHJKMN", "auth0|u1", time.Now().UTC())
	if !errors.Is(err, ErrAlreadyRedeemedByUser) {
		t.Fatalf("RedeemWithCAS same-user twice: got %v, want ErrAlreadyRedeemedByUser", err)
	}
	if !tx.rolledBack {
		t.Error("a unique-violation insert must roll back")
	}
	if tx.committed {
		t.Error("a rolled-back transaction should not also report committed")
	}
}

// TestPostgresStore_RedeemWithCAS_AlreadyRedeemed confirms that when the insert
// succeeds but the UPDATE matches no rows (ErrNoRows) and the follow-up
// existence check finds the code, ErrAlreadyRedeemed is returned and the
// transaction rolls back (undoing the wasted insert).
func TestPostgresStore_RedeemWithCAS_AlreadyRedeemed(t *testing.T) {
	t.Parallel()
	tx := &fakeTx{
		execResults: []error{nil}, // insert succeeds
		rowResults: []pgx.Row{
			&fakeErrorRow{err: pgx.ErrNoRows}, // UPDATE matched nothing
			&fakeIntRow{value: 1},             // code exists
		},
	}
	store := NewPostgresStore(&fakeOfferCodeQuerier{tx: tx})
	err := store.RedeemWithCAS(context.Background(), "ABCDEFGHJKMN", "auth0|u1", time.Now().UTC())
	if !errors.Is(err, ErrAlreadyRedeemed) {
		t.Fatalf("RedeemWithCAS already redeemed: got %v, want ErrAlreadyRedeemed", err)
	}
	if !tx.rolledBack {
		t.Error("a fully-consumed code must roll back the wasted insert")
	}
}

// TestPostgresStore_RedeemWithCAS_NotFound confirms that a foreign-key
// violation on the offer_code_redemptions insert — the code does not exist,
// so it can't be referenced — surfaces as ErrNotFound and rolls back.
func TestPostgresStore_RedeemWithCAS_NotFound(t *testing.T) {
	t.Parallel()
	tx := &fakeTx{
		execResults: []error{&pgconn.PgError{Code: pgForeignKeyViolationCode}},
	}
	store := NewPostgresStore(&fakeOfferCodeQuerier{tx: tx})
	err := store.RedeemWithCAS(context.Background(), "ZZZZZZZZZZZZ", "auth0|u1", time.Now().UTC())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("RedeemWithCAS not found: got %v, want ErrNotFound", err)
	}
	if !tx.rolledBack {
		t.Error("a foreign-key-violation insert must roll back")
	}
}

// TestPostgresStore_RedeemWithCAS_UpdateFindsNoCode is a defensive-code test
// for the (in production unreachable, since the insert's FK check already
// guarantees the code exists within the same transaction) branch where the
// UPDATE matches no rows and the follow-up existence check also finds no row:
// ErrNotFound is still returned rather than the wrong ErrAlreadyRedeemed.
func TestPostgresStore_RedeemWithCAS_UpdateFindsNoCode(t *testing.T) {
	t.Parallel()
	tx := &fakeTx{
		execResults: []error{nil},
		rowResults: []pgx.Row{
			&fakeErrorRow{err: pgx.ErrNoRows}, // UPDATE matched nothing
			&fakeErrorRow{err: pgx.ErrNoRows}, // existence check also finds nothing
		},
	}
	store := NewPostgresStore(&fakeOfferCodeQuerier{tx: tx})
	err := store.RedeemWithCAS(context.Background(), "ABCDEFGHJKMN", "auth0|u1", time.Now().UTC())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("RedeemWithCAS: got %v, want ErrNotFound", err)
	}
}

// TestPostgresStore_RedeemWithCAS_BeginError confirms a failure to start the
// transaction is wrapped and returned.
func TestPostgresStore_RedeemWithCAS_BeginError(t *testing.T) {
	t.Parallel()
	boom := errors.New("connection pool exhausted")
	store := NewPostgresStore(&fakeOfferCodeQuerier{beginErr: boom})
	err := store.RedeemWithCAS(context.Background(), "ABCDEFGHJKMN", "auth0|u1", time.Now().UTC())
	if !errors.Is(err, boom) {
		t.Fatalf("RedeemWithCAS begin error: got %v, want wrapped %v", err, boom)
	}
}

// ---------------------------------------------------------------------------
// RedeemedByUserID / RedeemedByUsers
// ---------------------------------------------------------------------------

// fakeRedeemedRow scans the 5-column join projection used by
// RedeemedByUserID/RedeemedByUsers: code, tier, duration_days, redeemed_at,
// user_id.
type fakeRedeemedRow struct {
	code         string
	tier         string
	durationDays int
	redeemedAt   *time.Time
	userID       *string
}

func (r *fakeRedeemedRow) Scan(dest ...any) error {
	if len(dest) != 5 {
		return fmt.Errorf("fakeRedeemedRow: expected 5 scan destinations, got %d", len(dest))
	}
	*dest[0].(*string) = r.code
	*dest[1].(*string) = r.tier
	*dest[2].(*int) = r.durationDays
	*dest[3].(**time.Time) = r.redeemedAt
	*dest[4].(**string) = r.userID
	return nil
}

type fakeRedeemedRows struct {
	rows []fakeRedeemedRow
	idx  int
}

func (r *fakeRedeemedRows) Next() bool { return r.idx < len(r.rows) }
func (r *fakeRedeemedRows) Scan(dest ...any) error {
	err := r.rows[r.idx].Scan(dest...)
	r.idx++
	return err
}
func (r *fakeRedeemedRows) Close()                                       {}
func (r *fakeRedeemedRows) Err() error                                   { return nil }
func (r *fakeRedeemedRows) Values() ([]any, error)                       { return nil, nil }
func (r *fakeRedeemedRows) RawValues() [][]byte                          { return nil }
func (r *fakeRedeemedRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRedeemedRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeRedeemedRows) Conn() *pgx.Conn                              { return nil }

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

// TestPostgresStore_RedeemedByUserID_Hit confirms a redemption row joined with
// its code's tier/duration hydrates correctly.
func TestPostgresStore_RedeemedByUserID_Hit(t *testing.T) {
	t.Parallel()
	redeemedAt := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	userID := "auth0|u1"
	q := &fakeOfferCodeQuerier{queryRows: &fakeRedeemedRows{rows: []fakeRedeemedRow{
		{code: "ABCDEFGHJKMN", tier: "Pro", durationDays: 30, redeemedAt: &redeemedAt, userID: &userID},
	}}}
	store := NewPostgresStore(q)

	got, err := store.RedeemedByUserID(context.Background(), "auth0|u1")
	if err != nil {
		t.Fatalf("RedeemedByUserID: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("count: got %d, want 1", len(got))
	}
	if got[0].Code != "ABCDEFGHJKMN" || got[0].Tier != profiles.TierPro || got[0].DurationDays != 30 {
		t.Errorf("row: got %+v", got[0])
	}
	if got[0].RedeemedAt == nil || !got[0].RedeemedAt.Equal(redeemedAt) {
		t.Errorf("RedeemedAt: got %v, want %v", got[0].RedeemedAt, redeemedAt)
	}
}

// ---------------------------------------------------------------------------
// RedeemedByUsers (batched)
// ---------------------------------------------------------------------------

// TestPostgresStore_RedeemedByUsers_Empty short-circuits an empty user set with
// no query and an empty, non-nil map.
func TestPostgresStore_RedeemedByUsers_Empty(t *testing.T) {
	t.Parallel()
	q := &fakeOfferCodeQuerier{}
	store := NewPostgresStore(q)

	got, err := store.RedeemedByUsers(context.Background(), nil)
	if err != nil {
		t.Fatalf("RedeemedByUsers: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("empty users: got %v, want empty non-nil map", got)
	}
}

// TestPostgresStore_RedeemedByUsers_PropagatesQueryError wraps a Query failure.
func TestPostgresStore_RedeemedByUsers_PropagatesQueryError(t *testing.T) {
	t.Parallel()
	boom := errors.New("redemptions boom")
	q := &fakeOfferCodeQuerier{queryErr: boom}
	store := NewPostgresStore(q)

	_, err := store.RedeemedByUsers(context.Background(), []string{"auth0|u1"})
	if !errors.Is(err, boom) {
		t.Fatalf("got %v, want wrapped %v", err, boom)
	}
}

// TestPostgresStore_RedeemedByUsers_GroupsByUser groups the flat result by each
// row's UserID so each user gets exactly their own redemptions.
func TestPostgresStore_RedeemedByUsers_GroupsByUser(t *testing.T) {
	t.Parallel()
	u1 := "auth0|u1"
	u2 := "auth0|u2"
	redeemed := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	q := &fakeOfferCodeQuerier{queryRows: &fakeRedeemedRows{rows: []fakeRedeemedRow{
		{code: "AAAAAAAAAAAA", tier: "Pro", durationDays: 30, redeemedAt: &redeemed, userID: &u1},
		{code: "BBBBBBBBBBBB", tier: "Personal", durationDays: 7, redeemedAt: &redeemed, userID: &u2},
		{code: "CCCCCCCCCCCC", tier: "Pro", durationDays: 90, redeemedAt: &redeemed, userID: &u1},
	}}}
	store := NewPostgresStore(q)

	got, err := store.RedeemedByUsers(context.Background(), []string{u1, u2})
	if err != nil {
		t.Fatalf("RedeemedByUsers: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("map size: got %d, want 2", len(got))
	}
	if len(got[u1]) != 2 {
		t.Errorf("u1 codes: got %d, want 2", len(got[u1]))
	}
	if len(got[u2]) != 1 || got[u2][0].Code != "BBBBBBBBBBBB" {
		t.Errorf("u2 codes: got %+v, want [BBBBBBBBBBBB]", got[u2])
	}
}

// TestPostgresStore_RedeemedByUsers_IgnoresNilUserID defends against a
// malformed/anonymised row slipping through the ANY($1) predicate: a nil
// UserID must never panic or be grouped.
func TestPostgresStore_RedeemedByUsers_IgnoresNilUserID(t *testing.T) {
	t.Parallel()
	q := &fakeOfferCodeQuerier{queryRows: &fakeRedeemedRows{rows: []fakeRedeemedRow{
		{code: "AAAAAAAAAAAA", tier: "Pro", durationDays: 30, redeemedAt: nil, userID: nil},
	}}}
	store := NewPostgresStore(q)

	got, err := store.RedeemedByUsers(context.Background(), []string{"auth0|u1"})
	if err != nil {
		t.Fatalf("RedeemedByUsers: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("map size: got %d, want 0 (nil-user row must be dropped)", len(got))
	}
}

// ---------------------------------------------------------------------------
// AnonymiseRedemptionsByUserID
// ---------------------------------------------------------------------------

// TestPostgresStore_AnonymiseRedemptionsByUserID_Success confirms both scrub
// statements run and the transaction commits.
func TestPostgresStore_AnonymiseRedemptionsByUserID_Success(t *testing.T) {
	t.Parallel()
	tx := &fakeTx{execResults: []error{nil, nil}}
	store := NewPostgresStore(&fakeOfferCodeQuerier{tx: tx})
	if err := store.AnonymiseRedemptionsByUserID(context.Background(), "auth0|u1"); err != nil {
		t.Fatalf("AnonymiseRedemptionsByUserID: %v", err)
	}
	if !tx.committed {
		t.Error("successful anonymise must commit")
	}
	if tx.execIdx != 2 {
		t.Errorf("exec count: got %d, want 2 (child scrub + legacy scrub)", tx.execIdx)
	}
}

// TestPostgresStore_AnonymiseRedemptionsByUserID_ChildExecError confirms that a
// failure on the child-table scrub is wrapped, returned, and rolled back.
func TestPostgresStore_AnonymiseRedemptionsByUserID_ChildExecError(t *testing.T) {
	t.Parallel()
	boom := errors.New("db timeout")
	tx := &fakeTx{execResults: []error{boom}}
	store := NewPostgresStore(&fakeOfferCodeQuerier{tx: tx})
	if err := store.AnonymiseRedemptionsByUserID(context.Background(), "auth0|u1"); !errors.Is(err, boom) {
		t.Fatalf("AnonymiseRedemptionsByUserID child error: got %v, want wrapped %v", err, boom)
	}
	if !tx.rolledBack {
		t.Error("a failed child scrub must roll back")
	}
}

// TestPostgresStore_AnonymiseRedemptionsByUserID_LegacyExecError confirms that
// a failure on the legacy-column scrub (after the child scrub succeeded) is
// wrapped, returned, and rolled back — the child scrub must not be left
// committed on its own.
func TestPostgresStore_AnonymiseRedemptionsByUserID_LegacyExecError(t *testing.T) {
	t.Parallel()
	boom := errors.New("db timeout")
	tx := &fakeTx{execResults: []error{nil, boom}}
	store := NewPostgresStore(&fakeOfferCodeQuerier{tx: tx})
	if err := store.AnonymiseRedemptionsByUserID(context.Background(), "auth0|u1"); !errors.Is(err, boom) {
		t.Fatalf("AnonymiseRedemptionsByUserID legacy error: got %v, want wrapped %v", err, boom)
	}
	if !tx.rolledBack {
		t.Error("a failed legacy scrub must roll back the whole transaction")
	}
	if tx.committed {
		t.Error("must not commit when the legacy scrub fails")
	}
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

// fakeListedRow scans the 8-column projection used by List: code, tier,
// duration_days, created_at, label, max_redemptions, redemption_count,
// last_redeemed_at.
type fakeListedRow struct {
	code            string
	tier            string
	durationDays    int
	createdAt       time.Time
	label           *string
	maxRedemptions  int
	redemptionCount int
	lastRedeemedAt  *time.Time
}

func (r *fakeListedRow) Scan(dest ...any) error {
	if len(dest) != 8 {
		return fmt.Errorf("fakeListedRow: expected 8 scan destinations, got %d", len(dest))
	}
	*dest[0].(*string) = r.code
	*dest[1].(*string) = r.tier
	*dest[2].(*int) = r.durationDays
	*dest[3].(*time.Time) = r.createdAt
	*dest[4].(**string) = r.label
	*dest[5].(*int) = r.maxRedemptions
	*dest[6].(*int) = r.redemptionCount
	*dest[7].(**time.Time) = r.lastRedeemedAt
	return nil
}

type fakeListedRows struct {
	rows []fakeListedRow
	idx  int
}

func (r *fakeListedRows) Next() bool { return r.idx < len(r.rows) }
func (r *fakeListedRows) Scan(dest ...any) error {
	err := r.rows[r.idx].Scan(dest...)
	r.idx++
	return err
}
func (r *fakeListedRows) Close()                                       {}
func (r *fakeListedRows) Err() error                                   { return nil }
func (r *fakeListedRows) Values() ([]any, error)                       { return nil, nil }
func (r *fakeListedRows) RawValues() [][]byte                          { return nil }
func (r *fakeListedRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeListedRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeListedRows) Conn() *pgx.Conn                              { return nil }

// TestPostgresStore_List_QueryError wraps a Query failure.
func TestPostgresStore_List_QueryError(t *testing.T) {
	t.Parallel()
	boom := errors.New("db down")
	store := NewPostgresStore(&fakeOfferCodeQuerier{queryErr: boom})
	_, err := store.List(context.Background(), nil, 500)
	if !errors.Is(err, boom) {
		t.Fatalf("List query error: got %v, want wrapped %v", err, boom)
	}
}

// TestPostgresStore_List_EmptyReturnsNonNilSlice confirms an empty result set
// returns an empty, non-nil slice.
func TestPostgresStore_List_EmptyReturnsNonNilSlice(t *testing.T) {
	t.Parallel()
	store := NewPostgresStore(&fakeOfferCodeQuerier{})
	got, err := store.List(context.Background(), nil, 500)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("got %v, want empty non-nil slice", got)
	}
}

// TestPostgresStore_List_Hit confirms a row hydrates with its LastRedeemedAt
// projection.
func TestPostgresStore_List_Hit(t *testing.T) {
	t.Parallel()
	created := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	lastRedeemed := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	label := "creator-campaign"
	q := &fakeOfferCodeQuerier{queryRows: &fakeListedRows{rows: []fakeListedRow{
		{
			code: "ABCDEFGHJKMN", tier: "Pro", durationDays: 30, createdAt: created,
			label: &label, maxRedemptions: 3, redemptionCount: 2, lastRedeemedAt: &lastRedeemed,
		},
	}}}
	store := NewPostgresStore(q)

	got, err := store.List(context.Background(), nil, 500)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("count: got %d, want 1", len(got))
	}
	row := got[0]
	if row.Code != "ABCDEFGHJKMN" || row.Label != "creator-campaign" || row.MaxRedemptions != 3 || row.RedemptionCount != 2 {
		t.Errorf("row: got %+v", row)
	}
	if row.LastRedeemedAt == nil || !row.LastRedeemedAt.Equal(lastRedeemed) {
		t.Errorf("LastRedeemedAt: got %v, want %v", row.LastRedeemedAt, lastRedeemed)
	}
}

// TestPostgresStore_List_PassesFilterAndLimit confirms the label filter
// pointer and limit are forwarded as query arguments (nil filter -> nil arg,
// which the SQL's $1::text IS NULL check treats as "no filter").
func TestPostgresStore_List_PassesFilterAndLimit(t *testing.T) {
	t.Parallel()
	q := &fakeOfferCodeQuerier{}
	store := NewPostgresStore(q)

	label := "creator"
	if _, err := store.List(context.Background(), &label, 42); err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(q.gotArgs) != 2 {
		t.Fatalf("args: got %v, want 2 args", q.gotArgs)
	}
	if got, ok := q.gotArgs[0].(*string); !ok || got != &label {
		t.Errorf("arg[0] (label filter): got %v", q.gotArgs[0])
	}
	if got, ok := q.gotArgs[1].(int); !ok || got != 42 {
		t.Errorf("arg[1] (limit): got %v", q.gotArgs[1])
	}
}
