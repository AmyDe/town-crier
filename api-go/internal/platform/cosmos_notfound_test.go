package platform

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

func TestIsCosmosNotFound(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"404 response error", &azcore.ResponseError{StatusCode: http.StatusNotFound}, true},
		{"wrapped 404 response error", fmt.Errorf("read item: %w", &azcore.ResponseError{StatusCode: http.StatusNotFound}), true},
		{"409 response error", &azcore.ResponseError{StatusCode: http.StatusConflict}, false},
		{"plain error", errors.New("network blip"), false},
		{"nil error", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := IsCosmosNotFound(tc.err); got != tc.want {
				t.Errorf("IsCosmosNotFound(%v): got %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
