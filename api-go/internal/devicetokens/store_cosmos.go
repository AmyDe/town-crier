package devicetokens

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

// CosmosItems is the consumer-side slice of the DeviceRegistrations container
// the store uses: single-partition point read/upsert/delete keyed on
// (userId, token) plus a single-partition query for the per-user list the GDPR
// export needs. Defining it here keeps azcosmos types out of the store's unit
// tests, which substitute a hand-written fake. platform.CosmosContainer
// satisfies it structurally.
type CosmosItems interface {
	ReadItem(ctx context.Context, partitionKey, id string) ([]byte, error)
	UpsertItem(ctx context.Context, partitionKey string, item []byte) error
	DeleteItem(ctx context.Context, partitionKey, id string) error
	QueryItems(ctx context.Context, partitionKey, query string, params map[string]any) ([][]byte, error)
}

// CosmosStore reads and writes device registrations in the DeviceRegistrations
// container. It holds only the consumer-side item interface, so no SDK type
// leaks past it.
//
// Partition strategy: the container is partitioned by /userId and the document
// id is the token, so every operation is a single-partition point operation
// keyed on (userId, token). The export's per-user list is a single-partition
// query — never cross-partition.
type CosmosStore struct {
	items CosmosItems
}

// NewCosmosStore returns a store backed by the given Cosmos item accessor.
func NewCosmosStore(items CosmosItems) *CosmosStore {
	return &CosmosStore{items: items}
}

// GetByToken point-reads the registration for (userID, token). A missing
// document returns (nil, nil) — the "not registered yet" signal the PUT handler
// branches on, mirroring .NET's null return; any other failure is wrapped.
func (s *CosmosStore) GetByToken(ctx context.Context, userID, token string) (*DeviceRegistration, error) {
	raw, err := s.items.ReadItem(ctx, userID, token)
	if err != nil {
		if isNotFound(err) {
			return nil, nil //nolint:nilnil // absent registration is a valid "not found" signal, not an error
		}
		return nil, fmt.Errorf("read device token %q: %w", token, err)
	}
	var doc deviceDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("decode device token %q: %w", token, err)
	}
	reg, err := doc.toDomain()
	if err != nil {
		return nil, fmt.Errorf("hydrate device token %q: %w", token, err)
	}
	return &reg, nil
}

// Save upserts the registration document (partition key == user id, id == token).
func (s *CosmosStore) Save(ctx context.Context, reg DeviceRegistration) error {
	body, err := json.Marshal(newDeviceDocument(reg))
	if err != nil {
		return fmt.Errorf("encode device token %q: %w", reg.Token, err)
	}
	if err := s.items.UpsertItem(ctx, reg.UserID, body); err != nil {
		return fmt.Errorf("upsert device token %q: %w", reg.Token, err)
	}
	return nil
}

// Delete removes the registration for (userID, token). A 404 is tolerated so the
// operation is idempotent — the token may already be gone (prior call or TTL),
// matching .NET DeleteByTokenAsync.
func (s *CosmosStore) Delete(ctx context.Context, userID, token string) error {
	if err := s.items.DeleteItem(ctx, userID, token); err != nil {
		if isNotFound(err) {
			return nil
		}
		return fmt.Errorf("delete device token %q: %w", token, err)
	}
	return nil
}

// ListByUser returns every registration in the user's partition, for the GDPR
// export. Single-partition query keyed on the user id, mirroring .NET
// GetByUserIdAsync.
func (s *CosmosStore) ListByUser(ctx context.Context, userID string) ([]DeviceRegistration, error) {
	const query = "SELECT * FROM c WHERE c.userId = @userId"
	items, err := s.items.QueryItems(ctx, userID, query, map[string]any{"@userId": userID})
	if err != nil {
		return nil, fmt.Errorf("query device tokens for %q: %w", userID, err)
	}
	regs := make([]DeviceRegistration, 0, len(items))
	for _, raw := range items {
		var doc deviceDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("decode device token list for %q: %w", userID, err)
		}
		reg, err := doc.toDomain()
		if err != nil {
			return nil, fmt.Errorf("hydrate device token list for %q: %w", userID, err)
		}
		regs = append(regs, reg)
	}
	return regs, nil
}

// idOnlyDocument captures just the id projected by the cascade-delete query
// (SELECT c.id FROM c ...), so the cascade need not hydrate full documents.
type idOnlyDocument struct {
	ID string `json:"id"`
}

// DeleteAllByUserID removes every device registration in the user's partition: it
// queries the partition for the document ids, then point-deletes each. Used by
// the account-deletion cascade (dormant cleanup and DELETE /v1/me), mirroring
// .NET CosmosDeviceRegistrationRepository.DeleteAllByUserIdAsync. All operations
// are single-partition; a 404 on an individual delete is tolerated (idempotent).
func (s *CosmosStore) DeleteAllByUserID(ctx context.Context, userID string) error {
	const query = "SELECT c.id FROM c WHERE c.userId = @userId"
	raws, err := s.items.QueryItems(ctx, userID, query, map[string]any{"@userId": userID})
	if err != nil {
		return fmt.Errorf("query device token ids for %q: %w", userID, err)
	}
	for _, raw := range raws {
		var doc idOnlyDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return fmt.Errorf("decode device token id for %q: %w", userID, err)
		}
		if err := s.items.DeleteItem(ctx, userID, doc.ID); err != nil && !isNotFound(err) {
			return fmt.Errorf("delete device token %q for %q: %w", doc.ID, userID, err)
		}
	}
	return nil
}

// isNotFound reports whether err is a Cosmos 404 response.
func isNotFound(err error) bool {
	var respErr *azcore.ResponseError
	return errors.As(err, &respErr) && respErr.StatusCode == http.StatusNotFound
}
