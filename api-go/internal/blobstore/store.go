// Package blobstore is a thin, consumer-facing wrapper over the official azblob
// SDK for the Town Crier share-card cache (#738 Slice 3, ADR 0037). It exposes
// only the Get/Put a keyed-PNG object cache needs and never leaks SDK types past
// its methods. Authentication is the pinned user-assigned managed identity
// (AZURE_CLIENT_ID); there is no shared-key / connection-string path, mirroring
// the Service Bus and Postgres identity model. Connections open lazily on first
// call, so wiring can construct it without paying a cold-start cost.
package blobstore

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
)

// ErrMissingAccountURL is returned when NewStore is called without a blob
// account URL. The empty-means-unwired convention lives with the caller: it must
// not construct a Store at all when SHARE_CARDS_BLOB_URL is unset, so the seam
// stays a genuine nil interface and the og:image handler regenerates on demand.
var ErrMissingAccountURL = errors.New("blob account url is required")

// maxCardBytes bounds a downloaded card. A baked 1200x630 PNG is a few hundred
// KiB; this is a generous safety cap against a runaway body read.
const maxCardBytes = 8 << 20 // 8 MiB

// blobAPI is the consumer-side seam over the two azblob operations Store needs.
// The production implementation (azblobAPI) wraps *azblob.Client; tests supply a
// hand-written fake. download returns the raw SDK error, unwrapped, so Store.Get
// can classify a not-found via bloberror.HasCode. The unexported methods keep the
// seam private to this package.
type blobAPI interface {
	download(ctx context.Context, container, key string) (io.ReadCloser, error)
	upload(ctx context.Context, container, key string, data []byte) error
}

// Store reads and writes keyed PNG cards in one blob container.
type Store struct {
	api       blobAPI
	container string
}

// NewStore builds a share-card blob store for accountURL/container, authenticated
// by the pinned user-assigned managed identity (azureClientID). When
// azureClientID is empty the SDK falls back to the ambient managed identity.
// accountURL must be non-empty (see ErrMissingAccountURL).
func NewStore(accountURL, container, azureClientID string) (*Store, error) {
	if accountURL == "" {
		return nil, ErrMissingAccountURL
	}

	credOpts := &azidentity.ManagedIdentityCredentialOptions{}
	if azureClientID != "" {
		credOpts.ID = azidentity.ClientID(azureClientID)
	}
	cred, err := azidentity.NewManagedIdentityCredential(credOpts)
	if err != nil {
		return nil, fmt.Errorf("build managed-identity credential: %w", err)
	}

	client, err := azblob.NewClient(accountURL, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("build blob client: %w", err)
	}

	return &Store{api: azblobAPI{client: client}, container: container}, nil
}

// Get returns the cached card for key. A missing blob is a cache MISS —
// (nil, false, nil), never an error — so the caller regenerates. Only a genuine
// failure (network, auth, other) returns a non-nil error.
func (s *Store) Get(ctx context.Context, key string) ([]byte, bool, error) {
	body, err := s.api.download(ctx, s.container, key)
	if err != nil {
		if bloberror.HasCode(err, bloberror.BlobNotFound) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("download share card %q: %w", key, err)
	}
	defer func() { _ = body.Close() }()

	data, err := io.ReadAll(io.LimitReader(body, maxCardBytes))
	if err != nil {
		return nil, false, fmt.Errorf("read share card %q: %w", key, err)
	}
	return data, true, nil
}

// Put stores png under key, overwriting any existing card for that key.
func (s *Store) Put(ctx context.Context, key string, png []byte) error {
	if err := s.api.upload(ctx, s.container, key, png); err != nil {
		return fmt.Errorf("upload share card %q: %w", key, err)
	}
	return nil
}

// azblobAPI is the production blobAPI backed by *azblob.Client. It preserves the
// raw SDK error on download so Store.Get can classify a not-found.
type azblobAPI struct {
	client *azblob.Client
}

func (a azblobAPI) download(ctx context.Context, container, key string) (io.ReadCloser, error) {
	resp, err := a.client.DownloadStream(ctx, container, key, nil)
	if err != nil {
		// Return the raw SDK error unwrapped so Store.Get can classify a not-found
		// via bloberror.HasCode; Store.Get adds the context on a genuine failure.
		return nil, err
	}
	return resp.Body, nil
}

func (a azblobAPI) upload(ctx context.Context, container, key string, data []byte) error {
	// Store.Put wraps this with the key context; return it raw here.
	_, err := a.client.UploadBuffer(ctx, container, key, data, nil)
	return err
}
