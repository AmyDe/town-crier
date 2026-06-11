package auth

import (
	"context"
	"log/slog"
	"testing"

	"github.com/auth0/go-jwt-middleware/v3/validator"
)

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

func TestNewAuth0Validator_DenyAllWhenUnconfigured(t *testing.T) {
	t.Parallel()

	// With no Auth0 domain/audience (the dev Go app's current env), the
	// validator must still deny every token rather than fail open. Validation
	// returns an error without any network call, so all protected routes 401 —
	// matching the contract-test reality until infra wires the env vars (it3+).
	tests := []struct {
		name     string
		domain   string
		audience string
	}{
		{"both empty", "", ""},
		{"domain only", "town-crier.eu.auth0.com", ""},
		{"audience only", "", "https://api.towncrierapp.uk"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			v, err := NewAuth0Validator(tc.domain, tc.audience, discardLogger())
			if err != nil {
				t.Fatalf("NewAuth0Validator: %v", err)
			}
			if _, err := v.ValidateToken(context.Background(), "any-token"); err == nil {
				t.Error("ValidateToken succeeded with no Auth0 config; want deny-all error")
			}
		})
	}
}

func TestNewAuth0Validator_ConfiguredBuildsRealValidator(t *testing.T) {
	t.Parallel()

	// A fully configured validator constructs without error. It does not reach
	// the network here (JWKS is fetched lazily on first ValidateToken), so this
	// asserts only that wiring the issuer URL and audience succeeds.
	v, err := NewAuth0Validator("town-crier.eu.auth0.com", "https://api.towncrierapp.uk", discardLogger())
	if err != nil {
		t.Fatalf("NewAuth0Validator: %v", err)
	}
	if v == nil {
		t.Fatal("NewAuth0Validator returned nil validator")
	}
}

func TestSubjectFromClaims(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		claims  any
		want    string
		wantErr bool
	}{
		{
			name: "valid claims expose sub",
			claims: &validator.ValidatedClaims{
				RegisteredClaims: validator.RegisteredClaims{Subject: "auth0|abc123"},
			},
			want: "auth0|abc123",
		},
		{
			name: "empty subject is rejected",
			claims: &validator.ValidatedClaims{
				RegisteredClaims: validator.RegisteredClaims{Subject: ""},
			},
			wantErr: true,
		},
		{
			name:    "unexpected claims type is rejected",
			claims:  "not claims",
			wantErr: true,
		},
		{
			name:    "nil claims is rejected",
			claims:  nil,
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := subjectFromClaims(tc.claims)
			if (err != nil) != tc.wantErr {
				t.Fatalf("subjectFromClaims err = %v, wantErr = %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("subject = %q, want %q", got, tc.want)
			}
		})
	}
}
