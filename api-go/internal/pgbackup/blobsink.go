package pgbackup

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

// ErrMissingBlobConfig is returned when the account URL or container is empty.
// The backup must fail closed rather than run without a destination.
var ErrMissingBlobConfig = errors.New("backup blob url and container are required")

// BlobSink uploads objects to one blob container with the pinned user-assigned
// managed identity. It sets no access tier: the account default (Cool) applies.
type BlobSink struct {
	client    *azblob.Client
	container string
}

// NewBlobSink builds a BlobSink. An empty azureClientID falls back to the
// ambient managed identity.
func NewBlobSink(accountURL, container, azureClientID string) (*BlobSink, error) {
	if accountURL == "" || container == "" {
		return nil, ErrMissingBlobConfig
	}
	opts := &azidentity.ManagedIdentityCredentialOptions{}
	if azureClientID != "" {
		opts.ID = azidentity.ClientID(azureClientID)
	}
	cred, err := azidentity.NewManagedIdentityCredential(opts)
	if err != nil {
		return nil, fmt.Errorf("build managed-identity credential: %w", err)
	}
	client, err := azblob.NewClient(accountURL, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("build blob client: %w", err)
	}
	return &BlobSink{client: client, container: container}, nil
}

// Put streams body to name, overwriting any existing blob.
func (b *BlobSink) Put(ctx context.Context, name string, body io.Reader) error {
	if _, err := b.client.UploadStream(ctx, b.container, name, body, nil); err != nil {
		return fmt.Errorf("upload blob %q: %w", name, err)
	}
	return nil
}
