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

	s := string(got)

	// HTML-significant characters must NOT be escaped (no <, >, &).
	for _, escaped := range []string{`<`, `>`, `&`} {
		if strings.Contains(s, escaped) {
			t.Errorf("output %q contains escaped sequence %q; SetEscapeHTML(false) not applied", s, escaped)
		}
	}
	for _, raw := range []string{"<", ">", "&"} {
		if !strings.Contains(s, raw) {
			t.Errorf("output %q is missing raw character %q", s, raw)
		}
	}

	// json.Encoder appends a trailing newline; EncodeJSON must trim it.
	if strings.HasSuffix(s, "\n") {
		t.Errorf("output %q has a trailing newline; it should be trimmed", s)
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
