// Package subscriptionevents detects tier-changing subscription saves by
// diffing a profile's stored subscription state before and after the change,
// and sends one anonymous JSON event (plus whole-base subscription totals) to
// each webhook configured in SUBSCRIPTION_WEBHOOKS.
//
// The payload never carries a user ID, email, name, Auth0 subject,
// transaction ID, product ID, offer code or price.
package subscriptionevents
