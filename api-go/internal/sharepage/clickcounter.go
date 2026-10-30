package sharepage

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

const (
	clickFlushInterval = time.Minute
	clickFlushTimeout  = 5 * time.Second
	clickMaxKeys       = 10_000
)

// ClickTotal is one row of a flush: the visits to an application from a source
// group during a UTC day, added to any stored total.
type ClickTotal struct {
	Day           time.Time
	AuthoritySlug string
	Ref           string
	Source        SourceGroup
	Count         int64
}

type clickKey struct {
	Day    time.Time
	Slug   string
	Ref    string
	Source SourceGroup
}

type clickWriter interface {
	WriteClicks(ctx context.Context, clicks []ClickTotal) error
}

// ClickCounter aggregates visits in a bounded in-memory map and writes them in
// batches. Add never does I/O, so it is safe on the request path. When the map is
// full, visits to keys it does not already hold are dropped and the number
// dropped is logged at the next flush.
type ClickCounter struct {
	writer   clickWriter
	now      func() time.Time
	logger   *slog.Logger
	interval time.Duration
	maxKeys  int

	mu      sync.Mutex
	pending map[clickKey]int64
	dropped int64
}

// NewClickCounter returns a ClickCounter that writes through writer. Start it
// with Run.
func NewClickCounter(writer clickWriter, now func() time.Time, logger *slog.Logger) *ClickCounter {
	return &ClickCounter{
		writer:   writer,
		now:      now,
		logger:   logger,
		interval: clickFlushInterval,
		maxKeys:  clickMaxKeys,
		pending:  make(map[clickKey]int64),
	}
}

// Add counts one visit against the current UTC day.
func (c *ClickCounter) Add(slug, ref string, source SourceGroup) {
	year, month, day := c.now().UTC().Date()
	key := clickKey{Day: time.Date(year, month, day, 0, 0, 0, 0, time.UTC), Slug: slug, Ref: ref, Source: source}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.merge(key, 1)
}

// Run flushes on an interval until ctx is cancelled, then does one final flush
// bounded by its own timeout before it returns. Failed flushes are logged and
// their clicks kept for the next attempt.
func (c *ClickCounter) Run(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			c.flush(context.WithoutCancel(ctx))
			return
		case <-ticker.C:
			c.flush(ctx)
		}
	}
}

func (c *ClickCounter) merge(key clickKey, count int64) {
	if _, held := c.pending[key]; !held && len(c.pending) >= c.maxKeys {
		c.dropped += count
		return
	}
	c.pending[key] += count
}

func (c *ClickCounter) flush(ctx context.Context) {
	c.mu.Lock()
	pending := c.pending
	dropped := c.dropped
	c.pending = make(map[clickKey]int64)
	c.dropped = 0
	c.mu.Unlock()

	if dropped > 0 {
		c.logger.WarnContext(ctx, "share clicks dropped at capacity", "dropped", dropped)
	}
	if len(pending) == 0 {
		return
	}

	batch := make([]ClickTotal, 0, len(pending))
	for key, count := range pending {
		batch = append(batch, ClickTotal{Day: key.Day, AuthoritySlug: key.Slug, Ref: key.Ref, Source: key.Source, Count: count})
	}

	writeCtx, cancel := context.WithTimeout(ctx, clickFlushTimeout)
	defer cancel()
	if err := c.writer.WriteClicks(writeCtx, batch); err != nil {
		c.logger.ErrorContext(ctx, "share click flush failed", "rows", len(batch), "error", err)
		c.mu.Lock()
		defer c.mu.Unlock()
		for key, count := range pending {
			c.merge(key, count)
		}
	}
}
