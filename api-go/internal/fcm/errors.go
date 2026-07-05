package fcm

import "errors"

// ErrInvalidServiceAccount reports that the configured FCM service-account JSON
// is missing, malformed, or does not carry the RSA private key / client email /
// token URI a JWT-bearer token exchange needs.
var ErrInvalidServiceAccount = errors.New("fcm: invalid service account")

// ErrInvalidOptions reports that the FCM options are enabled but incomplete
// (missing project id or service-account JSON).
var ErrInvalidOptions = errors.New("fcm: invalid options")
