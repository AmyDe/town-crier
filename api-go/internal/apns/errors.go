package apns

import "errors"

// ErrInvalidAuthKey reports that the configured APNs auth key is not a valid
// PKCS8-encoded ECDSA private key (a malformed or non-EC .p8).
var ErrInvalidAuthKey = errors.New("apns: invalid auth key")
