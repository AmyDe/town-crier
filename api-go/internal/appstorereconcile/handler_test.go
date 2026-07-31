package appstorereconcile

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/appstoreserverapi"
)

// --- fakes ---

type fakeHistoryFetcher struct {
	items        []appstoreserverapi.NotificationHistoryItem
	err          error
	calls        int
	capturedFrom time.Time
	capturedTo   time.Time
}

func (f *fakeHistoryFetcher) FetchNotificationHistory(_ context.Context, start, end time.Time) ([]appstoreserverapi.NotificationHistoryItem, error) {
	f.calls++
	f.capturedFrom = start
	f.capturedTo = end
	return f.items, f.err
}

type fakeVerifier struct {
	results map[string]string
	errs    map[string]error
}

func newFakeVerifier() *fakeVerifier {
	return &fakeVerifier{results: map[string]string{}, errs: map[string]error{}}
}

func (f *fakeVerifier) VerifyAndDecode(signed string) (string, error) {
	if e, ok := f.errs[signed]; ok {
		return "", e
	}
	if j, ok := f.results[signed]; ok {
		return j, nil
	}
	return "", fmt.Errorf("unmapped test jws %q", signed)
}

type fakeIdempotencyChecker struct {
	processed map[string]bool
	err       error
}

func newFakeIdempotencyChecker() *fakeIdempotencyChecker {
	return &fakeIdempotencyChecker{processed: map[string]bool{}}
}

func (f *fakeIdempotencyChecker) IsProcessed(_ context.Context, uuid string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.processed[uuid], nil
}

// fakeApplier is a spy: it records every call so a test can assert it was
// never invoked in observe mode, and can be primed per-signedPayload with an
// error or a wasNew value.
type fakeApplier struct {
	calls  []string
	wasNew map[string]bool
	errs   map[string]error
}

func newFakeApplier() *fakeApplier {
	return &fakeApplier{wasNew: map[string]bool{}, errs: map[string]error{}}
}

func (f *fakeApplier) Process(_ context.Context, signedPayload string) (bool, error) {
	f.calls = append(f.calls, signedPayload)
	if e, ok := f.errs[signedPayload]; ok {
		return false, e
	}
	return f.wasNew[signedPayload], nil
}

// --- test helpers ---

func notificationJSON(notifType, subtype, uuid string) string {
	if subtype == "" {
		return fmt.Sprintf(`{"notificationType":%q,"notificationUUID":%q,"data":{"signedTransactionInfo":"INNER"}}`, notifType, uuid)
	}
	return fmt.Sprintf(`{"notificationType":%q,"subtype":%q,"notificationUUID":%q,"data":{"signedTransactionInfo":"INNER"}}`, notifType, subtype, uuid)
}

var testNow = time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)

const testLookback = 24 * time.Hour

type testDeps struct {
	fetcher     *fakeHistoryFetcher
	verifier    *fakeVerifier
	idempotency *fakeIdempotencyChecker
	applier     *fakeApplier
}

func newTestHandler(applyEnabled bool) (*Handler, *testDeps) {
	d := &testDeps{
		fetcher:     &fakeHistoryFetcher{},
		verifier:    newFakeVerifier(),
		idempotency: newFakeIdempotencyChecker(),
		applier:     newFakeApplier(),
	}
	h := New(d.fetcher, d.verifier, d.idempotency, d.applier, testLookback, applyEnabled,
		slog.New(slog.DiscardHandler), func() time.Time { return testNow })
	return h, d
}

// --- tests ---

func TestRun_ZeroItemsCycle(t *testing.T) {
	t.Parallel()
	h, d := newTestHandler(false)
	d.fetcher.items = nil

	scanned, gaps, applied, err := h.Run(context.Background())

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if scanned != 0 || gaps != 0 || applied != 0 {
		t.Errorf("scanned=%d gaps=%d applied=%d, want 0/0/0", scanned, gaps, applied)
	}
}

func TestRun_AllGapsInObserveModeNeverCallsApplier(t *testing.T) {
	t.Parallel()
	h, d := newTestHandler(false)
	d.fetcher.items = []appstoreserverapi.NotificationHistoryItem{
		{SignedPayload: "jws-1"},
		{SignedPayload: "jws-2"},
	}
	d.verifier.results["jws-1"] = notificationJSON("SUBSCRIBED", "", "uuid-1")
	d.verifier.results["jws-2"] = notificationJSON("DID_RENEW", "", "uuid-2")

	scanned, gaps, applied, err := h.Run(context.Background())

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if scanned != 2 {
		t.Errorf("scanned = %d, want 2", scanned)
	}
	if gaps != 2 {
		t.Errorf("gaps = %d, want 2", gaps)
	}
	if applied != 0 {
		t.Errorf("applied = %d, want 0 in observe mode", applied)
	}
	if len(d.applier.calls) != 0 {
		t.Fatalf("applier.calls = %v, want none called in observe mode", d.applier.calls)
	}
}

func TestRun_AllGapsInApplyModeCallsApplierOncePerGap(t *testing.T) {
	t.Parallel()
	h, d := newTestHandler(true)
	d.fetcher.items = []appstoreserverapi.NotificationHistoryItem{
		{SignedPayload: "jws-1"},
		{SignedPayload: "jws-2"},
	}
	d.verifier.results["jws-1"] = notificationJSON("SUBSCRIBED", "", "uuid-1")
	d.verifier.results["jws-2"] = notificationJSON("DID_RENEW", "", "uuid-2")
	d.applier.wasNew["jws-1"] = true
	d.applier.wasNew["jws-2"] = true

	scanned, gaps, applied, err := h.Run(context.Background())

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if scanned != 2 || gaps != 2 {
		t.Errorf("scanned=%d gaps=%d, want 2/2", scanned, gaps)
	}
	if applied != 2 {
		t.Errorf("applied = %d, want 2", applied)
	}
	if len(d.applier.calls) != 2 {
		t.Fatalf("applier.calls = %v, want exactly 2 calls (one per gap)", d.applier.calls)
	}
}

func TestRun_ApplyModeCountsAppliedOnlyWhenApplierReportsWasNew(t *testing.T) {
	t.Parallel()
	h, d := newTestHandler(true)
	d.fetcher.items = []appstoreserverapi.NotificationHistoryItem{
		{SignedPayload: "jws-1"},
	}
	d.verifier.results["jws-1"] = notificationJSON("DID_CHANGE_RENEWAL_PREF", "DOWNGRADE", "uuid-1")
	d.applier.wasNew["jws-1"] = false // e.g. a no-op downgrade event

	scanned, gaps, applied, err := h.Run(context.Background())

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if scanned != 1 || gaps != 1 {
		t.Errorf("scanned=%d gaps=%d, want 1/1", scanned, gaps)
	}
	if applied != 0 {
		t.Errorf("applied = %d, want 0 (applier reported wasNew=false)", applied)
	}
	if len(d.applier.calls) != 1 {
		t.Errorf("applier.calls = %v, want exactly 1 call", d.applier.calls)
	}
}

func TestRun_AllAlreadyProcessedIsZeroGapsRegardlessOfMode(t *testing.T) {
	t.Parallel()
	for _, applyEnabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("applyEnabled=%v", applyEnabled), func(t *testing.T) {
			t.Parallel()
			h, d := newTestHandler(applyEnabled)
			d.fetcher.items = []appstoreserverapi.NotificationHistoryItem{
				{SignedPayload: "jws-1"},
				{SignedPayload: "jws-2"},
			}
			d.verifier.results["jws-1"] = notificationJSON("SUBSCRIBED", "", "uuid-1")
			d.verifier.results["jws-2"] = notificationJSON("DID_RENEW", "", "uuid-2")
			d.idempotency.processed["uuid-1"] = true
			d.idempotency.processed["uuid-2"] = true

			scanned, gaps, applied, err := h.Run(context.Background())

			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if scanned != 2 {
				t.Errorf("scanned = %d, want 2", scanned)
			}
			if gaps != 0 {
				t.Errorf("gaps = %d, want 0 (already processed)", gaps)
			}
			if applied != 0 {
				t.Errorf("applied = %d, want 0", applied)
			}
			if len(d.applier.calls) != 0 {
				t.Errorf("applier.calls = %v, want none (no gaps to apply)", d.applier.calls)
			}
		})
	}
}

func TestRun_MixedProcessedAndUnprocessed(t *testing.T) {
	t.Parallel()
	h, d := newTestHandler(true)
	d.fetcher.items = []appstoreserverapi.NotificationHistoryItem{
		{SignedPayload: "jws-processed"},
		{SignedPayload: "jws-gap"},
	}
	d.verifier.results["jws-processed"] = notificationJSON("SUBSCRIBED", "", "uuid-processed")
	d.verifier.results["jws-gap"] = notificationJSON("DID_RENEW", "", "uuid-gap")
	d.idempotency.processed["uuid-processed"] = true
	d.applier.wasNew["jws-gap"] = true

	scanned, gaps, applied, err := h.Run(context.Background())

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if scanned != 2 {
		t.Errorf("scanned = %d, want 2", scanned)
	}
	if gaps != 1 {
		t.Errorf("gaps = %d, want 1", gaps)
	}
	if applied != 1 {
		t.Errorf("applied = %d, want 1", applied)
	}
	if len(d.applier.calls) != 1 || d.applier.calls[0] != "jws-gap" {
		t.Errorf("applier.calls = %v, want [jws-gap]", d.applier.calls)
	}
}

func TestRun_HistoryFetcherErrorPropagates(t *testing.T) {
	t.Parallel()
	h, d := newTestHandler(false)
	d.fetcher.err = errors.New("apple unreachable")

	scanned, gaps, applied, err := h.Run(context.Background())

	if err == nil {
		t.Fatal("Run: want error when HistoryFetcher fails, got nil")
	}
	if scanned != 0 || gaps != 0 || applied != 0 {
		t.Errorf("scanned=%d gaps=%d applied=%d, want 0/0/0 on fatal fetch error", scanned, gaps, applied)
	}
}

func TestRun_OneBadItemVerifyFailureIsSkippedWithoutFailingCycle(t *testing.T) {
	t.Parallel()
	h, d := newTestHandler(false)
	d.fetcher.items = []appstoreserverapi.NotificationHistoryItem{
		{SignedPayload: "jws-good-1"},
		{SignedPayload: "jws-bad"},
		{SignedPayload: "jws-good-2"},
	}
	d.verifier.results["jws-good-1"] = notificationJSON("SUBSCRIBED", "", "uuid-good-1")
	d.verifier.errs["jws-bad"] = errors.New("tampered jws")
	d.verifier.results["jws-good-2"] = notificationJSON("DID_RENEW", "", "uuid-good-2")

	scanned, gaps, applied, err := h.Run(context.Background())

	if err != nil {
		t.Fatalf("Run: want no error (bad item isolated), got %v", err)
	}
	if scanned != 3 {
		t.Errorf("scanned = %d, want 3", scanned)
	}
	if gaps != 2 {
		t.Errorf("gaps = %d, want 2 (bad item excluded, both good items are gaps)", gaps)
	}
	if applied != 0 {
		t.Errorf("applied = %d, want 0 in observe mode", applied)
	}
}

func TestRun_OneBadItemDecodeFailureIsSkippedWithoutFailingCycle(t *testing.T) {
	t.Parallel()
	h, d := newTestHandler(false)
	d.fetcher.items = []appstoreserverapi.NotificationHistoryItem{
		{SignedPayload: "jws-malformed"},
		{SignedPayload: "jws-good"},
	}
	// Missing the required notificationUUID field -> DecodeNotification fails.
	d.verifier.results["jws-malformed"] = `{"notificationType":"SUBSCRIBED","data":{"signedTransactionInfo":"INNER"}}`
	d.verifier.results["jws-good"] = notificationJSON("SUBSCRIBED", "", "uuid-good")

	scanned, gaps, applied, err := h.Run(context.Background())

	if err != nil {
		t.Fatalf("Run: want no error (malformed item isolated), got %v", err)
	}
	if scanned != 2 {
		t.Errorf("scanned = %d, want 2", scanned)
	}
	if gaps != 1 {
		t.Errorf("gaps = %d, want 1 (malformed item excluded)", gaps)
	}
	if applied != 0 {
		t.Errorf("applied = %d, want 0", applied)
	}
}

func TestRun_ApplierErrorIsSkippedWithoutFailingCycle(t *testing.T) {
	t.Parallel()
	h, d := newTestHandler(true)
	d.fetcher.items = []appstoreserverapi.NotificationHistoryItem{
		{SignedPayload: "jws-1"},
	}
	d.verifier.results["jws-1"] = notificationJSON("SUBSCRIBED", "", "uuid-1")
	d.applier.errs["jws-1"] = errors.New("save profile failed")

	scanned, gaps, applied, err := h.Run(context.Background())

	if err != nil {
		t.Fatalf("Run: want no error (applier failure isolated), got %v", err)
	}
	if scanned != 1 || gaps != 1 {
		t.Errorf("scanned=%d gaps=%d, want 1/1", scanned, gaps)
	}
	if applied != 0 {
		t.Errorf("applied = %d, want 0 (applier errored)", applied)
	}
}

func TestRun_WindowMatchesNowMinusLookback(t *testing.T) {
	t.Parallel()
	h, d := newTestHandler(false)

	if _, _, _, err := h.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	wantEnd := testNow
	wantStart := testNow.Add(-testLookback)
	if !d.fetcher.capturedTo.Equal(wantEnd) {
		t.Errorf("end = %v, want %v", d.fetcher.capturedTo, wantEnd)
	}
	if !d.fetcher.capturedFrom.Equal(wantStart) {
		t.Errorf("start = %v, want %v", d.fetcher.capturedFrom, wantStart)
	}
	if d.fetcher.calls != 1 {
		t.Errorf("fetcher calls = %d, want 1", d.fetcher.calls)
	}
}

func TestRun_IdempotencyCheckErrorIsSkippedWithoutFailingCycle(t *testing.T) {
	t.Parallel()
	h, d := newTestHandler(false)
	d.fetcher.items = []appstoreserverapi.NotificationHistoryItem{
		{SignedPayload: "jws-1"},
	}
	d.verifier.results["jws-1"] = notificationJSON("SUBSCRIBED", "", "uuid-1")
	d.idempotency.err = errors.New("postgres down")

	scanned, gaps, applied, err := h.Run(context.Background())

	if err != nil {
		t.Fatalf("Run: want no error (idempotency check failure isolated), got %v", err)
	}
	if scanned != 1 {
		t.Errorf("scanned = %d, want 1", scanned)
	}
	if gaps != 0 {
		t.Errorf("gaps = %d, want 0 (could not determine gap status)", gaps)
	}
	if applied != 0 {
		t.Errorf("applied = %d, want 0", applied)
	}
}
