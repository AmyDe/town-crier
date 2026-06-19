package tc

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// grantTiers are the tiers an admin grant may set.
var grantTiers = []string{"Free", "Personal", "Pro"}

const grantSubscriptionUsage = "Usage: tc grant-subscription --email <email> --tier <Free|Personal|Pro>"

// runGrantSubscription implements `tc grant-subscription`: validate the
// arguments and PUT them to /v1/admin/subscriptions, reporting the outcome.
func runGrantSubscription(ctx context.Context, client *Client, env Env, args *ParsedArgs) int {
	email, err := args.GetRequired("email")
	if err != nil {
		fmt.Fprintln(env.Err, "Missing required argument: --email")
		fmt.Fprintln(env.Err, grantSubscriptionUsage)
		return exitUsage
	}
	tier, err := args.GetRequired("tier")
	if err != nil {
		fmt.Fprintln(env.Err, "Missing required argument: --tier")
		fmt.Fprintln(env.Err, grantSubscriptionUsage)
		return exitUsage
	}

	normalizedTier, ok := normalizeTier(tier, grantTiers)
	if !ok {
		fmt.Fprintf(env.Err, "Invalid tier: %s. Must be one of: Free, Personal, Pro\n", tier)
		return exitUsage
	}
	tier = normalizedTier

	req := grantSubscriptionRequest{Email: email, Tier: tier}
	resp, err := client.Put(ctx, "/v1/admin/subscriptions", req)
	if err != nil {
		fmt.Fprintf(env.Err, "API error: %s\n", err)
		return exitRuntime
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		fmt.Fprintf(env.Err, "User not found: %s\n", email)
		return exitRuntime
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
		fmt.Fprintf(env.Err, "API error (%d): %s\n", resp.StatusCode, string(body))
		return exitRuntime
	}

	fmt.Fprintf(env.Out, "Subscription granted: %s -> %s\n", email, tier)
	return exitOK
}
