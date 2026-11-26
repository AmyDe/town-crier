// Package webhooks delivers a Pro user's alerts to one HTTPS URL that the user
// chooses, as signed JSON (payload schemaVersion 1) or as a Slack message. It
// owns the endpoint store, the delivery outbox, the wire formats and the HTTP
// client that is safe to point at a user-supplied URL.
//
// The URL and its host are user data that can carry a token, so they never
// appear in logs, spans or error messages. The endpoint id identifies a webhook.
package webhooks
