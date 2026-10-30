package sharepage

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"
)

var fixedDay = time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)

func fixedNow() time.Time { return fixedDay.Add(13 * time.Hour) }

type fakeClickWriter struct {
	mu      sync.Mutex
	err     error
	batches [][]ClickTotal
	written chan struct{}
}

func (f *fakeClickWriter) WriteClicks(_ context.Context, clicks []ClickTotal) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.written != nil {
		defer func() {
			select {
			case f.written <- struct{}{}:
			default:
			}
		}()
	}
	if f.err != nil {
		return f.err
	}
	f.batches = append(f.batches, clicks)
	return nil
}

func (f *fakeClickWriter) setErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

func (f *fakeClickWriter) batchCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.batches)
}

func (f *fakeClickWriter) allRows() []ClickTotal {
	f.mu.Lock()
	defer f.mu.Unlock()
	var rows []ClickTotal
	for _, b := range f.batches {
		rows = append(rows, b...)
	}
	return rows
}

func rowCount(rows []ClickTotal, slug, ref string, source SourceGroup) int64 {
	var total int64
	for _, r := range rows {
		if r.AuthoritySlug == slug && r.Ref == ref && r.Source == source {
			total += r.Count
		}
	}
	return total
}

func TestClickCounter_Flush_AggregatesIntoOneBatch(t *testing.T) {
	t.Parallel()
	writer := &fakeClickWriter{}
	c := NewClickCounter(writer, fixedNow, slog.New(slog.DiscardHandler))

	c.Add("croydon", "1/A", SourceSEO)
	c.Add("croydon", "1/A", SourceSEO)
	c.Add("croydon", "1/A", SourceNone)
	c.Add("adur", "2/B", SourceSEO)
	c.flush(t.Context())

	if got := writer.batchCount(); got != 1 {
		t.Fatalf("batches = %d, want 1", got)
	}
	rows := writer.allRows()
	if len(rows) != 3 {
		t.Fatalf("rows = %+v, want 3", rows)
	}
	if got := rowCount(rows, "croydon", "1/A", SourceSEO); got != 2 {
		t.Fatalf("croydon seo = %d, want 2", got)
	}
	for _, r := range rows {
		if !r.Day.Equal(fixedDay) {
			t.Fatalf("day = %v, want %v", r.Day, fixedDay)
		}
	}
}

func TestClickCounter_Add_SplitsAcrossUTCDays(t *testing.T) {
	t.Parallel()
	writer := &fakeClickWriter{}
	bst := time.FixedZone("BST", 3600)
	now := time.Date(2026, 10, 1, 0, 30, 0, 0, bst)
	c := NewClickCounter(writer, func() time.Time { return now }, slog.New(slog.DiscardHandler))

	c.Add("croydon", "1/A", SourceNone)
	now = now.Add(time.Hour)
	c.Add("croydon", "1/A", SourceNone)
	c.flush(t.Context())

	rows := writer.allRows()
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want one per UTC day", rows)
	}
}

func TestClickCounter_Flush_EmptyDoesNotWrite(t *testing.T) {
	t.Parallel()
	writer := &fakeClickWriter{}
	c := NewClickCounter(writer, fixedNow, slog.New(slog.DiscardHandler))
	c.flush(t.Context())
	if got := writer.batchCount(); got != 0 {
		t.Fatalf("batches = %d, want 0", got)
	}
}

func TestClickCounter_Flush_ClearsAfterSuccess(t *testing.T) {
	t.Parallel()
	writer := &fakeClickWriter{}
	c := NewClickCounter(writer, fixedNow, slog.New(slog.DiscardHandler))
	c.Add("croydon", "1/A", SourceNone)
	c.flush(t.Context())
	c.flush(t.Context())
	if got := writer.batchCount(); got != 1 {
		t.Fatalf("batches = %d, want 1", got)
	}
}

func TestClickCounter_Flush_FailureLogsAndRetainsForNextFlush(t *testing.T) {
	t.Parallel()
	writer := &fakeClickWriter{err: errors.New("db down")}
	logs := &capturingHandler{}
	c := NewClickCounter(writer, fixedNow, slog.New(logs))

	c.Add("croydon", "1/A", SourceNone)
	c.flush(t.Context())
	if !logs.find(slog.LevelError, "share click flush failed") {
		t.Fatal("want an error log for the failed flush")
	}

	writer.setErr(nil)
	c.Add("croydon", "1/A", SourceNone)
	c.flush(t.Context())
	if got := rowCount(writer.allRows(), "croydon", "1/A", SourceNone); got != 2 {
		t.Fatalf("count = %d, want 2", got)
	}
}

func TestClickCounter_Add_OverflowDropsNewKeysAndReportsCount(t *testing.T) {
	t.Parallel()
	writer := &fakeClickWriter{}
	logs := &capturingHandler{}
	c := NewClickCounter(writer, fixedNow, slog.New(logs))
	c.maxKeys = 2

	c.Add("a", "1", SourceNone)
	c.Add("b", "1", SourceNone)
	c.Add("c", "1", SourceNone)
	c.Add("d", "1", SourceNone)
	c.Add("a", "1", SourceNone)
	if got := len(c.pending); got != 2 {
		t.Fatalf("pending keys = %d, want 2", got)
	}
	c.flush(t.Context())

	rows := writer.allRows()
	if got := rowCount(rows, "a", "1", SourceNone); got != 2 {
		t.Fatalf("existing key count = %d, want 2", got)
	}
	if got := rowCount(rows, "c", "1", SourceNone); got != 0 {
		t.Fatalf("overflowed key count = %d, want 0", got)
	}
	if !logs.find(slog.LevelWarn, "share clicks dropped at capacity") {
		t.Fatal("want a warning for the dropped clicks")
	}
	if got := droppedLogged(logs); got != 2 {
		t.Fatalf("logged dropped = %d, want 2", got)
	}

	c.Add("e", "1", SourceNone)
	if got := len(c.pending); got != 1 {
		t.Fatalf("pending keys after flush = %d, want 1", got)
	}
}

func TestClickCounter_Flush_RetainedClicksStayWithinCap(t *testing.T) {
	t.Parallel()
	writer := &hookClickWriter{}
	c := NewClickCounter(writer, fixedNow, slog.New(slog.DiscardHandler))
	c.maxKeys = 2
	writer.onWrite = func() error {
		c.Add("x", "1", SourceNone)
		c.Add("y", "1", SourceNone)
		return errors.New("db down")
	}

	c.Add("a", "1", SourceNone)
	c.Add("b", "1", SourceNone)
	c.Add("b", "1", SourceNone)
	c.flush(t.Context())

	if got := len(c.pending); got != 2 {
		t.Fatalf("pending keys = %d, want the cap of 2", got)
	}
	if c.dropped != 3 {
		t.Fatalf("dropped = %d, want 3", c.dropped)
	}
}

type hookClickWriter struct {
	onWrite func() error
}

func (w *hookClickWriter) WriteClicks(context.Context, []ClickTotal) error {
	return w.onWrite()
}

func TestClickCounter_Run_FlushesOnIntervalAndAtShutdown(t *testing.T) {
	t.Parallel()
	writer := &fakeClickWriter{written: make(chan struct{}, 1)}
	c := NewClickCounter(writer, fixedNow, slog.New(slog.DiscardHandler))
	c.interval = time.Millisecond

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.Run(ctx)
	}()

	c.Add("croydon", "1/A", SourceNone)
	select {
	case <-writer.written:
	case <-time.After(5 * time.Second):
		t.Fatal("interval flush did not happen")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop after cancel")
	}
}

func TestClickCounter_Run_FinalFlushIgnoresCancelledContext(t *testing.T) {
	t.Parallel()
	writer := &ctxCheckingWriter{}
	c := NewClickCounter(writer, fixedNow, slog.New(slog.DiscardHandler))
	c.interval = time.Hour

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	c.Add("croydon", "1/A", SourceNone)
	c.Run(ctx)

	if writer.calls != 1 {
		t.Fatalf("writes = %d, want 1", writer.calls)
	}
	if writer.ctxErr != nil {
		t.Fatalf("final flush ctx err = %v, want a live context", writer.ctxErr)
	}
}

type ctxCheckingWriter struct {
	calls  int
	ctxErr error
}

func (w *ctxCheckingWriter) WriteClicks(ctx context.Context, _ []ClickTotal) error {
	w.calls++
	w.ctxErr = ctx.Err()
	return nil
}

func TestClickCounter_ConcurrentAdds_NoLostCounts(t *testing.T) {
	t.Parallel()
	writer := &fakeClickWriter{}
	c := NewClickCounter(writer, fixedNow, slog.New(slog.DiscardHandler))
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 100 {
				c.Add("croydon", "1/A", SourceNone)
			}
		})
	}
	wg.Go(func() { c.flush(t.Context()) })
	wg.Wait()
	c.flush(t.Context())
	if got := rowCount(writer.allRows(), "croydon", "1/A", SourceNone); got != 800 {
		t.Fatalf("count = %d, want 800", got)
	}
}

func droppedLogged(logs *capturingHandler) int64 {
	logs.mu.Lock()
	defer logs.mu.Unlock()
	var dropped int64
	for _, r := range logs.records {
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "dropped" {
				dropped += a.Value.Int64()
			}
			return true
		})
	}
	return dropped
}
