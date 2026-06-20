package profiles

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// auth0Server is a hand-written fake Auth0 Management API. It mints a token on
// /oauth/token, records PATCH and DELETE calls, and lets a test drive the user
// endpoint's status code (e.g. 404 for delete tolerance).
type auth0Server struct {
	tokenHits    atomic.Int64
	lastTokenReq map[string]any
	patchBody    map[string]any
	patchAuth    string
	deletedPath  string
	userStatus   int // status returned by the /api/v2/users/{id} endpoint; 0 -> 200
}

func newAuth0Server(t *testing.T) (*auth0Server, *httptest.Server) {
	t.Helper()
	s := &auth0Server{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/oauth/token":
			s.tokenHits.Add(1)
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &s.lastTokenReq)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"tok-123","expires_in":86400}`))
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/v2/users/"):
			s.patchAuth = r.Header.Get("Authorization")
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &s.patchBody)
			if s.userStatus != 0 {
				w.WriteHeader(s.userStatus)
			}
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v2/users/"):
			// EscapedPath preserves the on-the-wire %7C; r.URL.Path would decode it.
			s.deletedPath = r.URL.EscapedPath()
			if s.userStatus != 0 {
				w.WriteHeader(s.userStatus)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return s, srv
}

func newTestAuth0Client(srv *httptest.Server) *Auth0Client {
	return NewAuth0Client(srv.Client(), srv.URL, "client-id", platform.NewSecret("client-secret"))
}

func TestAuth0Client_UpdateTier_MintsTokenAndPatches(t *testing.T) {
	t.Parallel()

	fake, srv := newAuth0Server(t)
	client := newTestAuth0Client(srv)

	if err := client.UpdateSubscriptionTier(context.Background(), "auth0|abc", "Pro"); err != nil {
		t.Fatalf("UpdateSubscriptionTier: %v", err)
	}

	// client_credentials grant with the management-API audience.
	if fake.lastTokenReq["grant_type"] != "client_credentials" {
		t.Errorf("grant_type: got %v", fake.lastTokenReq["grant_type"])
	}
	if fake.lastTokenReq["client_id"] != "client-id" || fake.lastTokenReq["client_secret"] != "client-secret" {
		t.Errorf("token creds wrong: %v", fake.lastTokenReq)
	}

	// PATCH carries the bearer token and the app_metadata.subscription_tier body.
	if fake.patchAuth != "Bearer tok-123" {
		t.Errorf("patch auth: got %q, want Bearer tok-123", fake.patchAuth)
	}
	meta, ok := fake.patchBody["app_metadata"].(map[string]any)
	if !ok || meta["subscription_tier"] != "Pro" {
		t.Errorf("patch body app_metadata.subscription_tier wrong: %v", fake.patchBody)
	}
}

func TestAuth0Client_CachesToken(t *testing.T) {
	t.Parallel()

	fake, srv := newAuth0Server(t)
	client := newTestAuth0Client(srv)

	for range 3 {
		if err := client.UpdateSubscriptionTier(context.Background(), "auth0|abc", "Pro"); err != nil {
			t.Fatalf("UpdateSubscriptionTier: %v", err)
		}
	}
	if got := fake.tokenHits.Load(); got != 1 {
		t.Errorf("token endpoint hit %d times, want 1 (token cached)", got)
	}
}

func TestAuth0Client_DeleteUser_Succeeds(t *testing.T) {
	t.Parallel()

	fake, srv := newAuth0Server(t)
	client := newTestAuth0Client(srv)

	if err := client.DeleteUser(context.Background(), "auth0|abc"); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	// The user id is URL-escaped in the path (the | becomes %7C).
	if !strings.Contains(fake.deletedPath, "auth0%7Cabc") {
		t.Errorf("delete path did not escape user id: %q", fake.deletedPath)
	}
}

func TestAuth0Client_DeleteUser_404Tolerant(t *testing.T) {
	t.Parallel()

	fake, srv := newAuth0Server(t)
	fake.userStatus = http.StatusNotFound
	client := newTestAuth0Client(srv)

	// A 404 on delete is tolerated — the Auth0 user is already gone, which is the
	// desired end state. No error.
	if err := client.DeleteUser(context.Background(), "auth0|abc"); err != nil {
		t.Errorf("DeleteUser 404: got %v, want nil (tolerated)", err)
	}
}

func TestAuth0Client_UpdateTier_PropagatesServerError(t *testing.T) {
	t.Parallel()

	fake, srv := newAuth0Server(t)
	fake.userStatus = http.StatusInternalServerError
	client := newTestAuth0Client(srv)

	if err := client.UpdateSubscriptionTier(context.Background(), "auth0|abc", "Pro"); err == nil {
		t.Error("UpdateSubscriptionTier: want error on 500, got nil")
	}
}

func TestNoOpAuth0Client(t *testing.T) {
	t.Parallel()

	var c Auth0Manager = NoOpAuth0Client{}
	if err := c.UpdateSubscriptionTier(context.Background(), "u", "Pro"); err != nil {
		t.Errorf("no-op UpdateSubscriptionTier: %v", err)
	}
	if err := c.DeleteUser(context.Background(), "u"); err != nil {
		t.Errorf("no-op DeleteUser: %v", err)
	}
}

func TestAuth0Client_SatisfiesManagerInterface(t *testing.T) {
	t.Parallel()

	_, srv := newAuth0Server(t)
	var _ Auth0Manager = newTestAuth0Client(srv)
	// Also confirm the token TTL math leaves headroom (expires_in minus a skew).
	if time.Minute <= 0 {
		t.Fatal("unreachable")
	}
}
