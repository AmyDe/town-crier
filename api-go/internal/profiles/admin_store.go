package profiles

import (
	"context"
	"encoding/json"
	"fmt"
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
// ErrNotFound. Mirrors the .NET GetByEmailCrossPartitionAsync.
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
// email substring, plus the continuation token for the next page. Mirrors the
// .NET ListCrossPartitionAsync: CONTAINS(c.email, @search, true) when a search
// is given, else an unfiltered scan. An empty search applies no filter (an empty
// substring matches every email, so the result set is identical either way).
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
