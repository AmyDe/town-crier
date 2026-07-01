package blobstore

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
)

// fakeBlobAPI is a hand-written double for the azblob seam. It records the
// container/key each call receives so the path building is checkable, and can be
// seeded to return bytes, a not-found response, or a genuine error.
type fakeBlobAPI struct {
	downloadBody []byte
	downloadErr  error
	gotDLKey     string
	gotDLCont    string

	uploadErr  error
	uploadN    int
	gotUpKey   string
	gotUpCont  string
	gotUpBytes []byte
}

func (f *fakeBlobAPI) download(_ context.Context, container, key string) (io.ReadCloser, error) {
	f.gotDLCont, f.gotDLKey = container, key
	if f.downloadErr != nil {
		return nil, f.downloadErr
	}
	return io.NopCloser(&sliceReader{data: f.downloadBody}), nil
}

func (f *fakeBlobAPI) upload(_ context.Context, container, key string, data []byte) error {
	f.uploadN++
	f.gotUpCont, f.gotUpKey, f.gotUpBytes = container, key, data
	return f.uploadErr
}

// sliceReader is a minimal io.Reader over a byte slice so the fake body needs no
// bytes.Reader import churn; it reads once then reports EOF.
type sliceReader struct {
	data []byte
	off  int
}

func (r *sliceReader) Read(p []byte) (int, error) {
	if r.off >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.off:])
	r.off += n
	return n, nil
}

func newTestStore(api blobAPI) *Store {
	return &Store{api: api, container: "share-cards"}
}

func TestStoreGet_Hit(t *testing.T) {
	t.Parallel()
	fake := &fakeBlobAPI{downloadBody: []byte("PNGDATA")}
	s := newTestStore(fake)

	got, found, err := s.Get(t.Context(), "165/23/03456/FUL.png")

	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("found = false, want true on a hit")
	}
	if string(got) != "PNGDATA" {
		t.Errorf("bytes = %q, want PNGDATA", got)
	}
	if fake.gotDLCont != "share-cards" {
		t.Errorf("download container = %q, want share-cards", fake.gotDLCont)
	}
	if fake.gotDLKey != "165/23/03456/FUL.png" {
		t.Errorf("download key = %q, want 165/23/03456/FUL.png", fake.gotDLKey)
	}
}

func TestStoreGet_MissMapsToNotFound(t *testing.T) {
	t.Parallel()
	// A missing blob surfaces as a ResponseError carrying the BlobNotFound code;
	// the store must translate it into a plain cache miss, never an error.
	fake := &fakeBlobAPI{downloadErr: &azcore.ResponseError{
		ErrorCode:  string(bloberror.BlobNotFound),
		StatusCode: http.StatusNotFound,
	}}
	s := newTestStore(fake)

	got, found, err := s.Get(t.Context(), "165/missing.png")

	if err != nil {
		t.Fatalf("a cache miss must not be an error: %v", err)
	}
	if found {
		t.Error("found = true, want false on a miss")
	}
	if got != nil {
		t.Errorf("bytes = %v, want nil on a miss", got)
	}
}

func TestStoreGet_GenuineErrorPropagates(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("network unreachable")
	s := newTestStore(&fakeBlobAPI{downloadErr: sentinel})

	_, found, err := s.Get(t.Context(), "165/x.png")

	if err == nil {
		t.Fatal("want a non-nil error for a genuine failure")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error %v must wrap the underlying failure", err)
	}
	if found {
		t.Error("found = true, want false on error")
	}
}

func TestStorePut(t *testing.T) {
	t.Parallel()
	fake := &fakeBlobAPI{}
	s := newTestStore(fake)

	if err := s.Put(t.Context(), "165/23/03456/FUL.png", []byte("PNG")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if fake.uploadN != 1 {
		t.Errorf("upload calls = %d, want 1", fake.uploadN)
	}
	if fake.gotUpCont != "share-cards" || fake.gotUpKey != "165/23/03456/FUL.png" {
		t.Errorf("upload target = (%q,%q), want (share-cards, 165/23/03456/FUL.png)", fake.gotUpCont, fake.gotUpKey)
	}
	if string(fake.gotUpBytes) != "PNG" {
		t.Errorf("upload bytes = %q, want PNG", fake.gotUpBytes)
	}
}

func TestStorePut_ErrorWraps(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("upload rejected")
	s := newTestStore(&fakeBlobAPI{uploadErr: sentinel})

	err := s.Put(t.Context(), "k", []byte("x"))

	if !errors.Is(err, sentinel) {
		t.Errorf("Put error %v must wrap the underlying failure", err)
	}
}

func TestNewStore_EmptyAccountURL_Errors(t *testing.T) {
	t.Parallel()
	// Empty means unwired; the caller must not construct a Store at all. NewStore
	// guards against an empty account URL with a sentinel rather than building a
	// broken client.
	_, err := NewStore("", "share-cards", "")
	if !errors.Is(err, ErrMissingAccountURL) {
		t.Errorf("NewStore(\"\") error = %v, want ErrMissingAccountURL", err)
	}
}

func TestNewStore_BuildsAgainstAccountURL(t *testing.T) {
	t.Parallel()
	// A well-formed account URL yields a usable store (managed-identity credential
	// and blob client construction open no connections, so this needs no Azure).
	s, err := NewStore("https://sttowncrierdev.blob.core.windows.net", "share-cards", "")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if s == nil {
		t.Fatal("NewStore returned a nil store")
	}
	if s.container != "share-cards" {
		t.Errorf("container = %q, want share-cards", s.container)
	}
}
