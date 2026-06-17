package httputil_test

import (
	"strings"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/httputil"
)

func TestEncodeJSON_DoesNotEscapeHTMLOrAppendNewline(t *testing.T) {
	t.Parallel()

	v := map[string]string{"q": "a < b && c > d"}

	got, err := httputil.EncodeJSON(v)
	if err != nil {
		t.Fatalf("EncodeJSON returned error: %v", err)
	}

	// With SetEscapeHTML(false) the <, > and & characters must appear raw (the
	// default encoder would Unicode-escape them), and the trailing newline that
	// json.Encoder writes must be trimmed. Asserting the exact bytes covers both
	// behaviours at once.
	if want := `{"q":"a < b && c > d"}`; string(got) != want {
		t.Errorf("got %q, want %q", string(got), want)
	}

	if strings.HasSuffix(string(got), "\n") {
		t.Errorf("output %q has a trailing newline; it should be trimmed", string(got))
	}
}

func TestEncodeJSON_EncodesValue(t *testing.T) {
	t.Parallel()

	got, err := httputil.EncodeJSON(struct {
		Name string `json:"name"`
	}{Name: "GLA"})
	if err != nil {
		t.Fatalf("EncodeJSON returned error: %v", err)
	}

	if want := `{"name":"GLA"}`; string(got) != want {
		t.Errorf("got %q, want %q", string(got), want)
	}
}

func TestEncodeJSON_ReturnsErrorForUnencodableValue(t *testing.T) {
	t.Parallel()

	if _, err := httputil.EncodeJSON(make(chan int)); err == nil {
		t.Fatal("expected an error encoding an unsupported type, got nil")
	}
}
