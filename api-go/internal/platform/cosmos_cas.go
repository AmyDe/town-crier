package platform

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
)

// CAS sentinel errors returned by the etag-conditional operations. They classify
// the Cosmos precondition responses the polling lease store needs to distinguish
// expected races from genuine failures. The lease store adapter maps these onto
// its own outcome types.
var (
	// ErrCASConflict is a 409 from a create — the document already exists.
	ErrCASConflict = errors.New("cosmos create conflict")
	// ErrCASPreconditionFailed is a 412 from a conditional replace/delete — the
	// etag did not match.
	ErrCASPreconditionFailed = errors.New("cosmos precondition failed")
	// ErrCASNotFound is a 404 from a conditional delete — the document was absent.
	ErrCASNotFound = errors.New("cosmos item not found")
)

// ReadItemWithETag point-reads a document and returns its body and current etag.
// found is false (no error) when the document is absent (404), matching the
// lease store's "no lease yet" path. The etag is the opaque CAS token for a
// subsequent conditional replace/delete.
func (c *CosmosContainer) ReadItemWithETag(ctx context.Context, partitionKey, id string) (body []byte, etag string, found bool, err error) {
	err = traceCosmosOp(ctx, c, "ReadItem", func(ctx context.Context) error {
		resp, rerr := c.container.ReadItem(ctx, azcosmos.NewPartitionKeyString(partitionKey), id, nil)
		if rerr != nil {
			return rerr
		}
		body = resp.Value
		etag = string(resp.ETag)
		found = true
		return nil
	})
	if err != nil {
		if isCASNotFound(err) {
			return nil, "", false, nil
		}
		return nil, "", false, fmt.Errorf("read item %q: %w", id, err)
	}
	return body, etag, found, nil
}

// CreateItemReturningETag creates a document only if it does not already exist,
// returning the server-assigned etag. A 409 (the document already exists)
// surfaces as ErrCASConflict so the caller can treat it as "lost the create
// race" rather than a failure.
func (c *CosmosContainer) CreateItemReturningETag(ctx context.Context, partitionKey string, item []byte) (string, error) {
	var etag string
	err := traceCosmosOp(ctx, c, "CreateItem", func(ctx context.Context) error {
		resp, cerr := c.container.CreateItem(ctx, azcosmos.NewPartitionKeyString(partitionKey), item, nil)
		if cerr != nil {
			return cerr
		}
		etag = string(resp.ETag)
		return nil
	})
	if err != nil {
		if isCASStatus(err, http.StatusConflict) {
			return "", ErrCASConflict
		}
		return "", fmt.Errorf("create item: %w", err)
	}
	return etag, nil
}

// ReplaceItemWithETag replaces a document only if its current etag matches,
// returning the new etag. A 412 (etag mismatch) surfaces as
// ErrCASPreconditionFailed so the caller can treat it as "lost the replace race".
func (c *CosmosContainer) ReplaceItemWithETag(ctx context.Context, partitionKey, id string, item []byte, etag string) (string, error) {
	var newETag string
	err := traceCosmosOp(ctx, c, "ReplaceItem", func(ctx context.Context) error {
		e := azcore.ETag(etag)
		resp, rerr := c.container.ReplaceItem(ctx, azcosmos.NewPartitionKeyString(partitionKey), id, item, &azcosmos.ItemOptions{IfMatchEtag: &e})
		if rerr != nil {
			return rerr
		}
		newETag = string(resp.ETag)
		return nil
	})
	if err != nil {
		if isCASStatus(err, http.StatusPreconditionFailed) {
			return "", ErrCASPreconditionFailed
		}
		return "", fmt.Errorf("replace item %q: %w", id, err)
	}
	return newETag, nil
}

// DeleteItemWithETag deletes a document only if its current etag matches. A 404
// (absent) surfaces as ErrCASNotFound and a 412 (etag mismatch) as
// ErrCASPreconditionFailed, so the lease store can distinguish "already gone"
// from "lost the race".
func (c *CosmosContainer) DeleteItemWithETag(ctx context.Context, partitionKey, id, etag string) error {
	err := traceCosmosOp(ctx, c, "DeleteItem", func(ctx context.Context) error {
		e := azcore.ETag(etag)
		_, derr := c.container.DeleteItem(ctx, azcosmos.NewPartitionKeyString(partitionKey), id, &azcosmos.ItemOptions{IfMatchEtag: &e})
		return derr
	})
	switch {
	case err == nil:
		return nil
	case isCASNotFound(err):
		return ErrCASNotFound
	case isCASStatus(err, http.StatusPreconditionFailed):
		return ErrCASPreconditionFailed
	default:
		return fmt.Errorf("delete item %q: %w", id, err)
	}
}

// isCASNotFound reports whether err is a Cosmos 404.
func isCASNotFound(err error) bool { return isCASStatus(err, http.StatusNotFound) }

// isCASStatus reports whether err is a Cosmos ResponseError with the given HTTP
// status code.
func isCASStatus(err error, status int) bool {
	var respErr *azcore.ResponseError
	return errors.As(err, &respErr) && respErr.StatusCode == status
}
