// Package indexnow tells IndexNow-participating search engines which sitemap
// URLs are new or changed since the last accepted submission.
package indexnow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

const (
	// DefaultEndpoint is the shared IndexNow API entry point.
	DefaultEndpoint = "https://api.indexnow.org/IndexNow"

	maxURLsPerRequest = 10000
	maxAttempts       = 6
	verifyBackoff     = 30 * time.Second
	requestTimeout    = 30 * time.Second
	maxResponseBytes  = 4096
)

// ErrNotAccepted marks a submission IndexNow did not accept: a status other
// than 200/202, or 403 on every attempt.
var ErrNotAccepted = errors.New("indexnow submission not accepted")

// Pair is one sitemap URL and its lastmod ("" when it has none).
type Pair struct {
	Loc     string
	Lastmod string
}

// PairSource yields the current sitemap pairs.
type PairSource interface {
	Pairs(ctx context.Context) ([]Pair, error)
}

// StateStore persists the pair set last accepted by IndexNow, as loc -> lastmod.
// Load reports false when nothing has been stored yet.
type StateStore interface {
	Load(ctx context.Context) (prev map[string]string, found bool, err error)
	Save(ctx context.Context, current map[string]string) error
}

// Config holds the submission identity and the injectable transport pieces.
// Endpoint, Client and Sleep default to production values when zero.
type Config struct {
	Host     string
	Key      string
	Endpoint string
	Client   *http.Client
	Sleep    func(ctx context.Context, d time.Duration) error
}

// Result counts the URLs submitted and the unchanged ones skipped.
type Result struct {
	Submitted int
	Skipped   int
}

// Submitter diffs the sitemap against the stored set and submits what changed.
type Submitter struct {
	src    PairSource
	state  StateStore
	cfg    Config
	logger *slog.Logger
}

// New returns a Submitter over src and state.
func New(src PairSource, state StateStore, cfg Config, logger *slog.Logger) *Submitter {
	if cfg.Endpoint == "" {
		cfg.Endpoint = DefaultEndpoint
	}
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: requestTimeout}
	}
	if cfg.Sleep == nil {
		cfg.Sleep = sleepContext
	}
	return &Submitter{src: src, state: state, cfg: cfg, logger: logger}
}

// Submit sends new or changed URLs to IndexNow and, only once every batch is
// accepted, stores the current set so a failure resubmits on the next run.
// With no stored set every URL is submitted; deleted URLs never are.
func (s *Submitter) Submit(ctx context.Context) (Result, error) {
	pairs, err := s.src.Pairs(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("read sitemap pairs: %w", err)
	}
	if len(pairs) == 0 {
		return Result{}, errors.New("sitemap has no URLs")
	}
	prev, _, err := s.state.Load(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("load indexnow state: %w", err)
	}

	current := make(map[string]string, len(pairs))
	var changed []string
	for _, p := range pairs {
		current[p.Loc] = p.Lastmod
		if old, ok := prev[p.Loc]; !ok || old != p.Lastmod {
			changed = append(changed, p.Loc)
		}
	}
	res := Result{Submitted: len(changed), Skipped: len(pairs) - len(changed)}
	if len(changed) == 0 {
		return res, nil
	}

	for start := 0; start < len(changed); start += maxURLsPerRequest {
		end := min(start+maxURLsPerRequest, len(changed))
		if err := s.post(ctx, changed[start:end]); err != nil {
			return Result{Skipped: len(pairs)}, err
		}
	}
	if err := s.state.Save(ctx, current); err != nil {
		return res, fmt.Errorf("save indexnow state: %w", err)
	}
	return res, nil
}

// post submits one batch, retrying on 403: IndexNow answers 403 while it cannot
// yet verify the key file, which clears on its own.
func (s *Submitter) post(ctx context.Context, urls []string) error {
	payload, err := json.Marshal(map[string]any{
		"host":        s.cfg.Host,
		"key":         s.cfg.Key,
		"keyLocation": "https://" + s.cfg.Host + "/" + s.cfg.Key + ".txt",
		"urlList":     urls,
	})
	if err != nil {
		return fmt.Errorf("encode indexnow payload: %w", err)
	}

	var status int
	var body string
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		status, body, err = s.do(ctx, payload)
		if err != nil {
			return err
		}
		switch status {
		case http.StatusOK, http.StatusAccepted:
			return nil
		case http.StatusForbidden:
			s.logger.WarnContext(ctx, "indexnow verification pending", "attempt", attempt, "status", status, "body", body)
			if attempt < maxAttempts {
				if err := s.cfg.Sleep(ctx, verifyBackoff); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("%w: status %d: %s", ErrNotAccepted, status, body)
		}
	}
	return fmt.Errorf("%w: status %d after %d attempts: %s", ErrNotAccepted, status, maxAttempts, body)
}

func (s *Submitter) do(ctx context.Context, payload []byte) (int, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return 0, "", fmt.Errorf("build indexnow request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := s.cfg.Client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("post to indexnow: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return 0, "", fmt.Errorf("read indexnow response: %w", err)
	}
	return resp.StatusCode, string(raw), nil
}

func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
