package polling

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

var londonTZ = mustLondon()

func mustLondon() *time.Location {
	loc, err := time.LoadLocation("Europe/London")
	if err != nil {
		panic(err)
	}
	return loc
}

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
	return nil
}

// fakeCallLog emulates the advisory lock with a mutex held from Begin until
// Commit or Rollback.
type fakeCallLog struct {
	lock sync.Mutex
	mu   sync.Mutex
	rows []PlanItCall
	next int64
}

func (f *fakeCallLog) Begin(ctx context.Context) (callLogTx, error) {
	f.lock.Lock()
	return &fakeCallTx{log: f}, nil
}

func (f *fakeCallLog) Latest(_ context.Context) (*PlanItCall, error) { return f.latest(), nil }

func (f *fakeCallLog) latest() *PlanItCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.rows) == 0 {
		return nil
	}
	r := f.rows[len(f.rows)-1]
	return &r
}

func (f *fakeCallLog) CountBetween(_ context.Context, from, to time.Time) (int, error) {
	return f.count(from, to), nil
}

func (f *fakeCallLog) count(from, to time.Time) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.rows {
		if !r.At.Before(from) && r.At.Before(to) {
			n++
		}
	}
	return n
}

type fakeCallTx struct {
	log  *fakeCallLog
	done bool
}

func (t *fakeCallTx) Latest(ctx context.Context) (*PlanItCall, error) { return t.log.latest(), nil }

func (t *fakeCallTx) CountBetween(ctx context.Context, from, to time.Time) (int, error) {
	return t.log.count(from, to), nil
}

func (t *fakeCallTx) Insert(_ context.Context, c PlanItCall) (int64, error) {
	t.log.mu.Lock()
	defer t.log.mu.Unlock()
	t.log.next++
	c.ID = t.log.next
	t.log.rows = append(t.log.rows, c)
	return c.ID, nil
}

func (t *fakeCallTx) Finish(_ context.Context, id int64, r CallResult) error {
	t.log.mu.Lock()
	defer t.log.mu.Unlock()
	for i := range t.log.rows {
		if t.log.rows[i].ID == id {
			t.log.rows[i].Status = r.Status
			t.log.rows[i].Total = r.Total
			t.log.rows[i].RetryAfter = r.RetryAfter
		}
	}
	return nil
}

func (t *fakeCallTx) Commit(context.Context) error {
	if !t.done {
		t.done = true
		t.log.lock.Unlock()
	}
	return nil
}

func (t *fakeCallTx) Rollback(context.Context) error { return t.Commit(context.Background()) }

func newTestPacer(t *testing.T, now time.Time, cap int) (*Pacer, *fakeCallLog, *fakeClock) {
	t.Helper()
	log := &fakeCallLog{}
	clk := &fakeClock{now: now}
	p := NewPacer(log, PacerConfig{DailyCap: cap, MinSpacing: 60 * time.Second}, clk.Now, clk.Sleep)
	return p, log, clk
}

func okWork(total int) func(context.Context) (planit.FetchPageResult, error) {
	return func(context.Context) (planit.FetchPageResult, error) {
		return planit.FetchPageResult{Total: &total}, nil
	}
}

func d(h time.Duration) *time.Duration { return &h }

func TestBudgetDay_BoundariesEuropeLondon(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		at        time.Time
		wantStart time.Time
		wantEnd   time.Time
	}{
		{"17:59 belongs to the day that started yesterday 18:00", time.Date(2026, 6, 10, 17, 59, 0, 0, londonTZ),
			time.Date(2026, 6, 9, 18, 0, 0, 0, londonTZ), time.Date(2026, 6, 10, 18, 0, 0, 0, londonTZ)},
		{"18:01 starts a new day", time.Date(2026, 6, 10, 18, 1, 0, 0, londonTZ),
			time.Date(2026, 6, 10, 18, 0, 0, 0, londonTZ), time.Date(2026, 6, 11, 18, 0, 0, 0, londonTZ)},
		{"exactly 18:00 starts a new day", time.Date(2026, 6, 10, 18, 0, 0, 0, londonTZ),
			time.Date(2026, 6, 10, 18, 0, 0, 0, londonTZ), time.Date(2026, 6, 11, 18, 0, 0, 0, londonTZ)},
		{"after midnight stays in the evening's day", time.Date(2026, 6, 11, 2, 0, 0, 0, londonTZ),
			time.Date(2026, 6, 10, 18, 0, 0, 0, londonTZ), time.Date(2026, 6, 11, 18, 0, 0, 0, londonTZ)},
		{"utc input is converted", time.Date(2026, 6, 10, 17, 30, 0, 0, time.UTC), // 18:30 BST
			time.Date(2026, 6, 10, 18, 0, 0, 0, londonTZ), time.Date(2026, 6, 11, 18, 0, 0, 0, londonTZ)},
		{"spring forward day is 23h", time.Date(2026, 3, 29, 12, 0, 0, 0, londonTZ),
			time.Date(2026, 3, 28, 18, 0, 0, 0, londonTZ), time.Date(2026, 3, 29, 18, 0, 0, 0, londonTZ)},
		{"fall back day is 25h", time.Date(2026, 10, 25, 12, 0, 0, 0, londonTZ),
			time.Date(2026, 10, 24, 18, 0, 0, 0, londonTZ), time.Date(2026, 10, 25, 18, 0, 0, 0, londonTZ)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotStart, gotEnd := BudgetDay(tc.at)
			if !gotStart.Equal(tc.wantStart) || !gotEnd.Equal(tc.wantEnd) {
				t.Fatalf("BudgetDay = [%s, %s), want [%s, %s)", gotStart, gotEnd, tc.wantStart, tc.wantEnd)
			}
		})
	}
	s, e := BudgetDay(time.Date(2026, 3, 29, 12, 0, 0, 0, londonTZ))
	if e.Sub(s) != 23*time.Hour {
		t.Fatalf("spring-forward budget day = %s, want 23h", e.Sub(s))
	}
	s, e = BudgetDay(time.Date(2026, 10, 25, 12, 0, 0, 0, londonTZ))
	if e.Sub(s) != 25*time.Hour {
		t.Fatalf("fall-back budget day = %s, want 25h", e.Sub(s))
	}
}

func TestPacer_Do_RecordsSuccessRow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 10, 20, 0, 0, 0, londonTZ)
	p, log, _ := newTestPacer(t, now, 300)
	day := time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)

	res, err := p.Do(context.Background(), planit.WorkWindowStart, day, 2, okWork(742))
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if res.Total == nil || *res.Total != 742 {
		t.Fatalf("result total = %v", res.Total)
	}
	if len(log.rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(log.rows))
	}
	r := log.rows[0]
	if r.Work != string(planit.WorkWindowStart) || r.PageIndex != 2 || r.WindowDay == nil || !r.WindowDay.Equal(day) {
		t.Fatalf("row = %+v", r)
	}
	if r.Status == nil || *r.Status != 200 || r.Total == nil || *r.Total != 742 {
		t.Fatalf("row result = status %v total %v", r.Status, r.Total)
	}
}

func TestPacer_Do_SpacesCallsAtLeastMinSpacing(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 10, 20, 0, 0, 0, londonTZ)
	p, log, _ := newTestPacer(t, now, 300)
	for i := 0; i < 3; i++ {
		if _, err := p.Do(context.Background(), planit.WorkWindowStart, time.Time{}, i, okWork(1)); err != nil {
			t.Fatalf("Do %d: %v", i, err)
		}
	}
	for i := 1; i < len(log.rows); i++ {
		if gap := log.rows[i].At.Sub(log.rows[i-1].At); gap < 60*time.Second {
			t.Fatalf("gap %d = %s, want >= 60s", i, gap)
		}
	}
}

func TestPacer_Do_BudgetExhausted_NoRequestNoRow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 10, 20, 0, 0, 0, londonTZ)
	p, log, _ := newTestPacer(t, now, 300)
	for i := 0; i < 300; i++ {
		log.rows = append(log.rows, PlanItCall{ID: int64(i + 1), At: now.Add(-time.Duration(i+2) * time.Minute), Work: "window_start", Status: intp(200)})
	}
	calls := 0
	_, err := p.Do(context.Background(), planit.WorkWindowStart, time.Time{}, 0, func(context.Context) (planit.FetchPageResult, error) {
		calls++
		return planit.FetchPageResult{}, nil
	})
	if !errors.Is(err, ErrBudgetExhausted) {
		t.Fatalf("err = %v, want ErrBudgetExhausted", err)
	}
	if calls != 0 || len(log.rows) != 300 {
		t.Fatalf("calls=%d rows=%d, want 0 calls and 300 rows", calls, len(log.rows))
	}
}

func TestPacer_Do_BudgetDayRollsAt1800(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 10, 18, 1, 0, 0, londonTZ)
	p, log, _ := newTestPacer(t, now, 1)
	log.rows = append(log.rows, PlanItCall{ID: 1, At: time.Date(2026, 6, 10, 17, 59, 0, 0, londonTZ), Work: "window_start", Status: intp(200)})
	if _, err := p.Do(context.Background(), planit.WorkWindowStart, time.Time{}, 0, okWork(1)); err != nil {
		t.Fatalf("call after 18:00 should be in a fresh budget day: %v", err)
	}
	if _, err := p.Do(context.Background(), planit.WorkWindowStart, time.Time{}, 0, okWork(1)); !errors.Is(err, ErrBudgetExhausted) {
		t.Fatalf("second call err = %v, want ErrBudgetExhausted", err)
	}
}

func TestPacer_Do_FailedRequestStillCounts(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 10, 20, 0, 0, 0, londonTZ)
	p, _, _ := newTestPacer(t, now, 300)
	boom := &planit.HTTPError{StatusCode: 400}
	if _, err := p.Do(context.Background(), planit.WorkWindowStart, time.Time{}, 0, func(context.Context) (planit.FetchPageResult, error) {
		return planit.FetchPageResult{}, boom
	}); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the work error", err)
	}
	n, err := p.CallsToday(context.Background(), now)
	if err != nil || n != 1 {
		t.Fatalf("CallsToday = %d, %v; want 1", n, err)
	}
}

func TestPacer_Do_BackoffFromLatestRow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 10, 20, 0, 0, 0, londonTZ)
	tests := []struct {
		name        string
		row         PlanItCall
		wantBackoff time.Duration
	}{
		{"429 with retry-after", PlanItCall{Status: intp(429), RetryAfter: d(20 * time.Minute)}, 20 * time.Minute},
		{"429 capped at 3h", PlanItCall{Status: intp(429), RetryAfter: d(10 * time.Hour)}, 3 * time.Hour},
		{"429 without retry-after defaults to 15m", PlanItCall{Status: intp(429)}, 15 * time.Minute},
		{"403", PlanItCall{Status: intp(403)}, 24 * time.Hour},
		{"timeout (null status)", PlanItCall{}, 30 * time.Minute},
		{"502", PlanItCall{Status: intp(502)}, 30 * time.Minute},
		{"500", PlanItCall{Status: intp(500)}, 30 * time.Minute},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			row := tc.row
			row.ID, row.Work, row.At = 1, "window_start", now.Add(-time.Minute)

			p, log, _ := newTestPacer(t, now, 300)
			log.rows = append(log.rows, row)
			calls := 0
			_, err := p.Do(context.Background(), planit.WorkWindowStart, time.Time{}, 0, func(context.Context) (planit.FetchPageResult, error) {
				calls++
				return planit.FetchPageResult{}, nil
			})
			if !errors.Is(err, ErrBackedOff) {
				t.Fatalf("err = %v, want ErrBackedOff", err)
			}
			if calls != 0 || len(log.rows) != 1 {
				t.Fatalf("calls=%d rows=%d; want no request and no new row", calls, len(log.rows))
			}
			st, err := p.Backoff(context.Background(), now)
			if err != nil {
				t.Fatal(err)
			}
			if want := row.At.Add(tc.wantBackoff); !st.Active || !st.Until.Equal(want) {
				t.Fatalf("Backoff = %+v, want active until %s", st, want)
			}
		})
	}
}

func TestPacer_Do_BackoffExpiresAndNonBackoffStatusesPass(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 10, 20, 0, 0, 0, londonTZ)
	tests := []struct {
		name string
		row  PlanItCall
	}{
		{"429 expired", PlanItCall{Status: intp(429), RetryAfter: d(5 * time.Minute), At: now.Add(-6 * time.Minute)}},
		{"timeout expired", PlanItCall{At: now.Add(-31 * time.Minute)}},
		{"400 never backs off", PlanItCall{Status: intp(400), At: now.Add(-time.Second)}},
		{"aborted (status 0) never backs off", PlanItCall{Status: intp(0), At: now.Add(-time.Second)}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			row := tc.row
			row.ID, row.Work = 1, "window_start"
			p, log, _ := newTestPacer(t, now, 300)
			log.rows = append(log.rows, row)
			if _, err := p.Do(context.Background(), planit.WorkWindowStart, time.Time{}, 0, okWork(1)); err != nil {
				t.Fatalf("Do: %v", err)
			}
		})
	}
}

func TestPacer_Do_MapsResultsIntoRow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 10, 20, 0, 0, 0, londonTZ)
	tests := []struct {
		name       string
		err        error
		wantStatus *int
		wantRetry  *time.Duration
	}{
		{"429 with retry-after", &planit.RateLimitError{RetryAfter: d(90 * time.Second)}, intp(429), d(90 * time.Second)},
		{"429 without retry-after", &planit.RateLimitError{}, intp(429), nil},
		{"403", &planit.ForbiddenError{}, intp(403), nil},
		{"timeout", planitTimeout(), nil, nil},
		{"503", &planit.HTTPError{StatusCode: 503}, intp(503), nil},
		{"transport error", errors.New("dial tcp: refused"), nil, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p, log, _ := newTestPacer(t, now, 300)
			_, err := p.Do(context.Background(), planit.WorkWindowStart, time.Time{}, 0, func(context.Context) (planit.FetchPageResult, error) {
				return planit.FetchPageResult{}, tc.err
			})
			if !errors.Is(err, tc.err) {
				t.Fatalf("err = %v, want %v", err, tc.err)
			}
			r := log.rows[0]
			if (r.Status == nil) != (tc.wantStatus == nil) || (r.Status != nil && *r.Status != *tc.wantStatus) {
				t.Fatalf("status = %v, want %v", r.Status, tc.wantStatus)
			}
			if (r.RetryAfter == nil) != (tc.wantRetry == nil) || (r.RetryAfter != nil && *r.RetryAfter != *tc.wantRetry) {
				t.Fatalf("retry_after = %v, want %v", r.RetryAfter, tc.wantRetry)
			}
		})
	}
}

func TestPacer_Do_CancelledContextDoesNotBackOff(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 10, 20, 0, 0, 0, londonTZ)
	p, log, _ := newTestPacer(t, now, 300)
	ctx, cancel := context.WithCancel(context.Background())
	_, err := p.Do(ctx, planit.WorkWindowStart, time.Time{}, 0, func(context.Context) (planit.FetchPageResult, error) {
		cancel()
		return planit.FetchPageResult{}, context.Canceled
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if r := log.rows[0]; r.Status == nil || *r.Status != 0 {
		t.Fatalf("status = %v, want 0 (aborted, recorded but no backoff)", r.Status)
	}
	st, err := p.Backoff(context.Background(), now)
	if err != nil || st.Active {
		t.Fatalf("Backoff = %+v, %v; want inactive", st, err)
	}
}

func TestPacer_Do_ConcurrentCallersHoldSpacingAndCap(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 10, 20, 0, 0, 0, londonTZ)
	p, log, _ := newTestPacer(t, now, 5)

	var wg sync.WaitGroup
	var mu sync.Mutex
	sent, exhausted := 0, 0
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := p.Do(context.Background(), planit.WorkWindowStart, time.Time{}, 0, okWork(1))
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				sent++
			case errors.Is(err, ErrBudgetExhausted):
				exhausted++
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	if sent != 5 || exhausted != 7 {
		t.Fatalf("sent=%d exhausted=%d, want 5 and 7", sent, exhausted)
	}
	for i := 1; i < len(log.rows); i++ {
		if gap := log.rows[i].At.Sub(log.rows[i-1].At); gap < 60*time.Second {
			t.Fatalf("gap %d = %s, want >= 60s", i, gap)
		}
	}
}

func TestPacer_CallsToday_CountsCurrentBudgetDayOnly(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 10, 20, 0, 0, 0, londonTZ)
	p, log, _ := newTestPacer(t, now, 300)
	log.rows = []PlanItCall{
		{ID: 1, At: time.Date(2026, 6, 10, 17, 59, 0, 0, londonTZ)},
		{ID: 2, At: time.Date(2026, 6, 10, 18, 0, 0, 0, londonTZ)},
		{ID: 3, At: time.Date(2026, 6, 10, 19, 0, 0, 0, londonTZ)},
	}
	n, err := p.CallsToday(context.Background(), now)
	if err != nil || n != 2 {
		t.Fatalf("CallsToday = %d, %v; want 2", n, err)
	}
}

func intp(v int) *int { return &v }

func planitTimeout() error { return errors.Join(planit.ErrTimeout, errors.New("deadline")) }
