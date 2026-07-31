package appstoreserverapi

import "errors"

// ErrInvalidSigningKey reports that the configured .p8 signing key is not a
// valid PKCS8-encoded ECDSA private key.
var ErrInvalidSigningKey = errors.New("appstoreserverapi: invalid signing key")

// ErrInvalidOptions reports that Options is missing a required field (KeyID,
// IssuerID, or BundleID).
var ErrInvalidOptions = errors.New("appstoreserverapi: invalid options")

// ErrRateLimited reports that FetchNotificationHistory exhausted its bounded
// retry budget against repeated 429 responses.
var ErrRateLimited = errors.New("appstoreserverapi: rate limited")

// ErrUnexpectedStatus reports a non-2xx, non-429 response from the App Store
// Server API.
var ErrUnexpectedStatus = errors.New("appstoreserverapi: unexpected status")

// ErrTooManyPages reports that pagination exceeded the defensive max-page
// cap without hasMore ever reporting false — a malformed or adversarial
// response looping forever.
var ErrTooManyPages = errors.New("appstoreserverapi: too many pages")
