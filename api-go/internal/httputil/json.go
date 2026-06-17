// Package httputil holds small, cross-cutting helpers shared by the HTTP
// handlers. It deliberately carries no business logic.
package httputil

import (
	"bytes"
	"encoding/json"
)

// EncodeJSON renders v as JSON with HTML escaping disabled and the trailing
// newline that json.Encoder appends trimmed off. This matches the relaxed
// (non-HTML-escaping) byte output of the .NET web serializer, so the wire bytes
// stay identical across the handlers that emit JSON responses.
func EncodeJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
