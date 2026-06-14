package platform

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

func TestIsCASStatus_MatchesWrappedResponseError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		err    error
		status int
		want   bool
	}{
		{"conflict matches", &azcore.ResponseError{StatusCode: http.StatusConflict}, http.StatusConflict, true},
		{"wrapped conflict matches", fmt.Errorf("create: %w", &azcore.ResponseError{StatusCode: http.StatusConflict}), http.StatusConflict, true},
		{"precondition matches", &azcore.ResponseError{StatusCode: http.StatusPreconditionFailed}, http.StatusPreconditionFailed, true},
		{"not-found matches", &azcore.ResponseError{StatusCode: http.StatusNotFound}, http.StatusNotFound, true},
		{"wrong status does not match", &azcore.ResponseError{StatusCode: http.StatusConflict}, http.StatusNotFound, false},
		{"plain error does not match", errors.New("network blip"), http.StatusConflict, false},
		{"nil does not match", nil, http.StatusConflict, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isCASStatus(tc.err, tc.status); got != tc.want {
				t.Errorf("isCASStatus: got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsCASNotFound_OnlyMatches404(t *testing.T) {
	t.Parallel()
	if !isCASNotFound(&azcore.ResponseError{StatusCode: http.StatusNotFound}) {
		t.Error("404 should be CAS not-found")
	}
	if isCASNotFound(&azcore.ResponseError{StatusCode: http.StatusConflict}) {
		t.Error("409 must not be CAS not-found")
	}
}
