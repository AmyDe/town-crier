package profiles

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// adminItems is the consumer-side slice of the Cosmos container the admin store
// needs: upsert (grant writes a profile back) plus cross-partition query and
// paged query (the admin lookups span partitions — email and the full list are
// not the partition key). platform.CosmosContainer satisfies it structurally.
type adminItems interface {
	UpsertItem(ctx context.Context, partitionKey string, item []byte) error
	QueryItemsCrossPartition(ctx context.Context, query string, params map[string]any) ([][]byte, error)
	QueryPageCrossPartition(ctx context.Context, query string, params map[string]any, pageSize int, continuationToken string) ([][]byte, string, error)
}

// AdminStore serves the admin surface's cross-partition profile operations:
// find-by-email (used by the grant endpoint) and the paged user list. It is
// separate from CosmosStore because those operations span partitions, unlike the
// single-partition /v1/me lifecycle.
type AdminStore struct {
	items adminItems
}

// NewAdminStore returns an admin store backed by the given accessor.
func NewAdminStore(items adminItems) *AdminStore { return &AdminStore{items: items} }

// Page is one page of the admin user list: the profiles on this page plus the
// continuation token for the next page (empty when exhausted).
type Page struct {
	Profiles          []*UserProfile
	ContinuationToken string
}

// GetByEmail returns the first profile whose email matches exactly, or
// ErrNotFound.
func (s *AdminStore) GetByEmail(ctx context.Context, email string) (*UserProfile, error) {
	rows, err := s.items.QueryItemsCrossPartition(ctx,
		"SELECT * FROM c WHERE c.email = @email",
		map[string]any{"@email": email})
	if err != nil {
		return nil, fmt.Errorf("query profile by email: %w", err)
	}
	if len(rows) == 0 {
		return nil, ErrNotFound
	}
	var doc profileDocument
	if err := json.Unmarshal(rows[0], &doc); err != nil {
		return nil, fmt.Errorf("decode profile: %w", err)
	}
	return doc.toDomain()
}

// GetByOriginalTransactionID returns the profile whose stored Apple original
// transaction id matches, or ErrNotFound. App Store Server Notifications carry
// no user id, so the webhook locates the subscriber by this cross-partition
// lookup.
func (s *AdminStore) GetByOriginalTransactionID(ctx context.Context, originalTransactionID string) (*UserProfile, error) {
	rows, err := s.items.QueryItemsCrossPartition(ctx,
		"SELECT * FROM c WHERE c.originalTransactionId = @txnId",
		map[string]any{"@txnId": originalTransactionID})
	if err != nil {
		return nil, fmt.Errorf("query profile by original transaction id: %w", err)
	}
	if len(rows) == 0 {
		return nil, ErrNotFound
	}
	var doc profileDocument
	if err := json.Unmarshal(rows[0], &doc); err != nil {
		return nil, fmt.Errorf("decode profile: %w", err)
	}
	return doc.toDomain()
}

// ByDigestDay returns every profile whose configured digest day matches day —
// the weekly-digest candidate set, before per-user tier and preference gating.
// The digest day is bound as the int weekday value (Sunday=0 … Saturday=6),
// matching the int stored in the profile document's digestDay field.
func (s *AdminStore) ByDigestDay(ctx context.Context, day time.Weekday) ([]*UserProfile, error) {
	rows, err := s.items.QueryItemsCrossPartition(ctx,
		"SELECT * FROM c WHERE c.digestDay = @digestDay",
		map[string]any{"@digestDay": int(day)})
	if err != nil {
		return nil, fmt.Errorf("query profiles by digest day: %w", err)
	}
	profiles := make([]*UserProfile, 0, len(rows))
	for _, raw := range rows {
		var doc profileDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("decode profile: %w", err)
		}
		p, err := doc.toDomain()
		if err != nil {
			return nil, fmt.Errorf("hydrate profile: %w", err)
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
}

// Dormant returns every profile last active strictly before cutoff — the
// dormant-account set the cleanup worker erases (UK GDPR Art. 5(1)(e), ADR 0023).
// The cutoff comparison is done in Go on the parsed LastActiveAt rather than as a
// Cosmos string comparison: production documents carry lastActiveAt in two wire
// formats ("+00:00" and RFC 3339 "Z"), and "Z" sorts after "+", so a
// lexicographic SQL "<" would silently miss "Z"-stored dormant accounts. The
// scan is a once-a-day batch over a small user base, so hydrating all profiles
// and filtering in Go is both correct and cheap.
func (s *AdminStore) Dormant(ctx context.Context, cutoff time.Time) ([]*UserProfile, error) {
	rows, err := s.items.QueryItemsCrossPartition(ctx, "SELECT * FROM c", nil)
	if err != nil {
		return nil, fmt.Errorf("query dormant profiles: %w", err)
	}
	dormant := make([]*UserProfile, 0)
	for _, raw := range rows {
		var doc profileDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("decode profile: %w", err)
		}
		p, err := doc.toDomain()
		if err != nil {
			return nil, fmt.Errorf("hydrate profile: %w", err)
		}
		if p.LastActiveAt.Before(cutoff) {
			dormant = append(dormant, p)
		}
	}
	return dormant, nil
}

// LapsedPaid returns every profile whose stored tier is paid but whose
// entitlement has lapsed at now — i.e. EffectiveTier(now) has collapsed to Free.
// These are the profiles the daily subscription sweep (WORKER_MODE=subscription-
// sweep) reverts to the Free tier in Cosmos and syncs to Auth0. Like Dormant it
// does a full cross-partition scan and filters in Go: the lapsed test is the
// domain EffectiveTier rule (expiry-vs-grace comparison, across the two
// production timestamp wire formats), not a Cosmos predicate, and the scan is a
// once-a-day batch over a small user base. Far-future paid grants (pro-domain
// auto-grants, admin 2099 grants) keep their stored tier under EffectiveTier and
// are never selected; Free profiles and paid profiles still within their window
// (including a live grace period) are likewise skipped.
func (s *AdminStore) LapsedPaid(ctx context.Context, now time.Time) ([]*UserProfile, error) {
	rows, err := s.items.QueryItemsCrossPartition(ctx, "SELECT * FROM c", nil)
	if err != nil {
		return nil, fmt.Errorf("query lapsed paid profiles: %w", err)
	}
	lapsed := make([]*UserProfile, 0)
	for _, raw := range rows {
		var doc profileDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("decode profile: %w", err)
		}
		p, err := doc.toDomain()
		if err != nil {
			return nil, fmt.Errorf("hydrate profile: %w", err)
		}
		if p.Tier.IsPaid() && p.EffectiveTier(now) == TierFree {
			lapsed = append(lapsed, p)
		}
	}
	return lapsed, nil
}

// Save upserts the profile (id == user id == partition key).
func (s *AdminStore) Save(ctx context.Context, p *UserProfile) error {
	body, err := json.Marshal(newProfileDocument(p))
	if err != nil {
		return fmt.Errorf("encode profile %q: %w", p.UserID, err)
	}
	if err := s.items.UpsertItem(ctx, p.UserID, body); err != nil {
		return fmt.Errorf("upsert profile %q: %w", p.UserID, err)
	}
	return nil
}

// List returns one page of profiles, optionally filtered by a case-insensitive
// email substring, plus the continuation token for the next page. Uses
// CONTAINS(c.email, @search, true) when a search is given, else an unfiltered
// scan. An empty search applies no filter (an empty substring matches every
// email, so the result set is identical either way).
func (s *AdminStore) List(ctx context.Context, emailSearch string, pageSize int, continuationToken string) (Page, error) {
	query := "SELECT * FROM c"
	var params map[string]any
	if emailSearch != "" {
		query = "SELECT * FROM c WHERE CONTAINS(c.email, @search, true)"
		params = map[string]any{"@search": emailSearch}
	}

	rows, next, err := s.items.QueryPageCrossPartition(ctx, query, params, pageSize, continuationToken)
	if err != nil {
		return Page{}, fmt.Errorf("list profiles: %w", err)
	}

	page := Page{Profiles: make([]*UserProfile, 0, len(rows)), ContinuationToken: next}
	for _, raw := range rows {
		var doc profileDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return Page{}, fmt.Errorf("decode profile: %w", err)
		}
		p, err := doc.toDomain()
		if err != nil {
			return Page{}, fmt.Errorf("hydrate profile: %w", err)
		}
		page.Profiles = append(page.Profiles, p)
	}
	return page, nil
}
