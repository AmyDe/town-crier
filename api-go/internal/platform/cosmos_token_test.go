package platform

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

// fakeTokenCredential stands in for a managed-identity credential. delay models
// how long the identity endpoint round trip takes; the GetToken honours its
// ctx deadline so a test can prove whether the wrapper detached from the
// caller's per-try deadline.
type fakeTokenCredential struct {
	delay time.Duration
	token string
}

func (f fakeTokenCredential) GetToken(ctx context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	select {
	case <-time.After(f.delay):
		return azcore.AccessToken{Token: f.token, ExpiresOn: time.Now().Add(time.Hour)}, nil
	case <-ctx.Done():
		return azcore.AccessToken{}, ctx.Err()
	}
}

// TestDetachedTokenCredential_DetachesFromShortParentDeadline is the regression
// guard for tc-u7mp: on a cold Container Apps Job replica the first MI token
// fetch outlasts cosmosTryTimeout (the per-Cosmos-try deadline that wraps the
// auth policy), so every Cosmos attempt is cancelled before a token caches. The
// wrapper must run GetToken under its own generous timeout, detached from the
// caller's short deadline, so the cold fetch completes and the token is returned.
func TestDetachedTokenCredential_DetachesFromShortParentDeadline(t *testing.T) {
	t.Parallel()

	cred := detachedTokenCredential{
		inner:   fakeTokenCredential{delay: 100 * time.Millisecond, token: "tok"},
		timeout: 5 * time.Second,
	}

	// Parent deadline (10ms) is far shorter than the inner fetch (100ms). Without
	// detaching this would return context.DeadlineExceeded.
	parent, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	tok, err := cred.GetToken(parent, policy.TokenRequestOptions{})
	if err != nil {
		t.Fatalf("GetToken under short parent deadline: got err %v, want nil", err)
	}
	if tok.Token != "tok" {
		t.Errorf("token: got %q, want %q", tok.Token, "tok")
	}
}

// TestDetachedTokenCredential_BoundsAcquisition proves the detached fetch still
// has an upper bound: when the inner credential outlasts the wrapper's own
// timeout, GetToken returns a deadline-exceeded error rather than hanging forever.
func TestDetachedTokenCredential_BoundsAcquisition(t *testing.T) {
	t.Parallel()

	cred := detachedTokenCredential{
		inner:   fakeTokenCredential{delay: 200 * time.Millisecond, token: "tok"},
		timeout: 20 * time.Millisecond,
	}

	_, err := cred.GetToken(context.Background(), policy.TokenRequestOptions{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("GetToken with timeout shorter than inner delay: got err %v, want context.DeadlineExceeded", err)
	}
}
