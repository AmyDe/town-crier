package webhooks

// FailureReason classifies a failed delivery. The values are stored and served.
type FailureReason string

// FailureReason values.
const (
	// FailureGone means the receiver answered 410.
	FailureGone FailureReason = "gone"
	// FailureHTTPStatus means the receiver answered another non-2xx status,
	// including a redirect.
	FailureHTTPStatus FailureReason = "http_status"
	// FailureBlockedAddress means the host resolved to an address or port that
	// webhooks may not connect to.
	FailureBlockedAddress FailureReason = "blocked_address"
	// FailureTimeout means the receiver did not answer in time.
	FailureTimeout FailureReason = "timeout"
	// FailureTLS means the TLS handshake failed.
	FailureTLS FailureReason = "tls"
	// FailureConnection means any other transport failure.
	FailureConnection FailureReason = "connection"
)
