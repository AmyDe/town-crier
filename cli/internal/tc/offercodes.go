package tc

import (
	"bufio"
	"context"
	"fmt"
	"io"
)

const (
	minOfferCount        = 1
	maxOfferCount        = 1000
	minOfferDurationDays = 1
	maxOfferDurationDays = 365
)

// offerCodeTiers are the tiers an offer code may grant (Free is not allowed).
var offerCodeTiers = []string{"Personal", "Pro"}

const generateOfferCodesUsage = "Usage: tc generate-offer-codes --count <N> --tier <Personal|Pro> --duration-days <D>"

// runGenerateOfferCodes implements `tc generate-offer-codes`: validate the
// arguments, POST them to /v1/admin/offer-codes, stream the generated codes to
// stdout (one per line), and print a summary to stderr.
func runGenerateOfferCodes(ctx context.Context, client *Client, env Env, args *ParsedArgs) int {
	countStr, err := args.GetRequired("count")
	if err != nil {
		fmt.Fprintln(env.Err, "Missing required argument: --count")
		fmt.Fprintln(env.Err, generateOfferCodesUsage)
		return exitUsage
	}
	tier, err := args.GetRequired("tier")
	if err != nil {
		fmt.Fprintln(env.Err, "Missing required argument: --tier")
		fmt.Fprintln(env.Err, generateOfferCodesUsage)
		return exitUsage
	}
	durationDaysStr, err := args.GetRequired("duration-days")
	if err != nil {
		fmt.Fprintln(env.Err, "Missing required argument: --duration-days")
		fmt.Fprintln(env.Err, generateOfferCodesUsage)
		return exitUsage
	}

	count, ok := parseStrictInt(countStr)
	if !ok || count < minOfferCount || count > maxOfferCount {
		fmt.Fprintf(env.Err, "Invalid --count: must be an integer between %d and %d\n", minOfferCount, maxOfferCount)
		return exitUsage
	}

	normalizedTier, ok := normalizeTier(tier, offerCodeTiers)
	if !ok {
		fmt.Fprintf(env.Err, "Invalid tier: %s. Must be one of: Personal, Pro\n", tier)
		return exitUsage
	}
	tier = normalizedTier

	durationDays, ok := parseStrictInt(durationDaysStr)
	if !ok || durationDays < minOfferDurationDays || durationDays > maxOfferDurationDays {
		fmt.Fprintf(env.Err, "Invalid --duration-days: must be an integer between %d and %d\n", minOfferDurationDays, maxOfferDurationDays)
		return exitUsage
	}

	req := generateOfferCodesRequest{Count: count, Tier: tier, DurationDays: durationDays}
	resp, err := client.Post(ctx, "/v1/admin/offer-codes", req)
	if err != nil {
		fmt.Fprintf(env.Err, "API error: %s\n", err)
		return exitRuntime
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
		fmt.Fprintf(env.Err, "API error (%d): %s\n", resp.StatusCode, string(body))
		return exitRuntime
	}

	scanner := bufio.NewScanner(io.LimitReader(resp.Body, maxRespBytes))
	scanner.Buffer(make([]byte, 0, 64*1024), maxRespBytes)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		fmt.Fprintln(env.Out, line)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(env.Err, "API error: %s\n", err)
		return exitRuntime
	}

	fmt.Fprintf(env.Err, "Generated %d codes: %s tier, %d days duration\n", count, tier, durationDays)
	return exitOK
}
