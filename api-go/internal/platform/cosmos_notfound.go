package platform

import "net/http"

// IsCosmosNotFound reports whether err is (or wraps) a Cosmos 404 response.
// Store implementations use it to treat an absent document as a "not found"
// outcome rather than a failure. It mirrors the unexported isCASNotFound used
// by the etag-conditional operations but is exported for reuse across the
// per-feature Cosmos stores.
func IsCosmosNotFound(err error) bool {
	return isCASStatus(err, http.StatusNotFound)
}
