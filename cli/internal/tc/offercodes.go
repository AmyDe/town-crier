package tc

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
)

const (
	minOfferCount         = 1
	maxOfferCount         = 1000
	minOfferDurationDays  = 1
	maxOfferDurationDays  = 365
	minMaxRedemptions     = 1
	maxMaxRedemptions     = 10000
	defaultMaxRedemptions = 1
)

// offerCodeTiers are the tiers an offer code may grant (Free is not allowed).
var offerCodeTiers = []string{"Personal", "Pro"}

const generateOfferCodesUsage = "Usage: tc generate-offer-codes --count <N> --tier <Personal|Pro> --duration-days <D> --label <label> [--max-redemptions <M>]"

// runGenerateOfferCodes implements `tc generate-offer-codes`: validate the
// arguments, POST them to /v1/admin/offer-codes, stream the generated codes to
// stdout (one per line), and print a summary to stderr. Every code in the
// batch shares one label and one redemption cap (--max-redemptions, default 1
// — the single-use case).
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
	label, err := args.GetRequired("label")
	if err != nil {
		fmt.Fprintln(env.Err, "Missing required argument: --label")
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

	if strings.TrimSpace(label) == "" {
		fmt.Fprintln(env.Err, "Invalid --label: must not be blank")
		return exitUsage
	}

	maxRedemptions := defaultMaxRedemptions
	var maxRedemptionsPtr *int
	if raw, hasMaxRedemptions := args.GetOptional("max-redemptions"); hasMaxRedemptions {
		n, ok := parseStrictInt(raw)
		if !ok || n < minMaxRedemptions || n > maxMaxRedemptions {
			fmt.Fprintf(env.Err, "Invalid --max-redemptions: must be an integer between %d and %d\n", minMaxRedemptions, maxMaxRedemptions)
			return exitUsage
		}
		maxRedemptions = n
		maxRedemptionsPtr = &n
	}

	req := generateOfferCodesRequest{
		Count: count, Tier: tier, DurationDays: durationDays,
		Label: label, MaxRedemptions: maxRedemptionsPtr,
	}
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

	fmt.Fprintf(env.Err, "Generated %d codes: %s tier, %d days duration, label %q, max %d redemptions\n",
		count, tier, durationDays, label, maxRedemptions)
	return exitOK
}

// runListOfferCodes implements `tc list-offer-codes [--label <substring>]`:
// fetch the code listing from GET /v1/admin/offer-codes and render it as a
// table. There is no pagination beyond the API's own default limit — the
// table is admin-minted and small.
func runListOfferCodes(ctx context.Context, client *Client, env Env, args *ParsedArgs) int {
	path := "/v1/admin/offer-codes"
	if label, hasLabel := args.GetOptional("label"); hasLabel {
		path += "?label=" + url.QueryEscape(label)
	}

	var items listOfferCodesResponse
	if err := client.GetJSON(ctx, path, &items); err != nil {
		fmt.Fprintln(env.Err, err.Error())
		return exitRuntime
	}

	printOfferCodesTable(env.Out, items)
	return exitOK
}

// printOfferCodesTable renders the listing as an aligned table: code, label,
// tier, duration, redeemed-x-of-N, created, last redeemed.
func printOfferCodesTable(out io.Writer, items listOfferCodesResponse) {
	const (
		hCode         = "Code"
		hLabel        = "Label"
		hTier         = "Tier"
		hDuration     = "Duration"
		hRedeemed     = "Redeemed"
		hCreated      = "Created"
		hLastRedeemed = "LastRedeemed"
	)

	type cells struct {
		code, label, tier, duration, redeemed, created, lastRedeemed string
	}
	rows := make([]cells, 0, len(items))
	for _, item := range items {
		created := item.CreatedAt
		rows = append(rows, cells{
			code:         item.Code,
			label:        item.Label,
			tier:         item.Tier,
			duration:     strconv.Itoa(item.DurationDays) + "d",
			redeemed:     fmt.Sprintf("%d/%d", item.RedemptionCount, item.MaxRedemptions),
			created:      datePart(&created),
			lastRedeemed: datePart(item.LastRedeemedAt),
		})
	}

	wCode, wLabel, wTier := len(hCode), len(hLabel), len(hTier)
	wDuration, wRedeemed := len(hDuration), len(hRedeemed)
	wCreated, wLastRedeemed := len(hCreated), len(hLastRedeemed)
	for _, r := range rows {
		wCode = max(wCode, len(r.code))
		wLabel = max(wLabel, len(r.label))
		wTier = max(wTier, len(r.tier))
		wDuration = max(wDuration, len(r.duration))
		wRedeemed = max(wRedeemed, len(r.redeemed))
		wCreated = max(wCreated, len(r.created))
		wLastRedeemed = max(wLastRedeemed, len(r.lastRedeemed))
	}

	format := fmt.Sprintf("%%-%ds  %%-%ds  %%-%ds  %%-%ds  %%-%ds  %%-%ds  %%-%ds\n",
		wCode, wLabel, wTier, wDuration, wRedeemed, wCreated, wLastRedeemed)

	header := fmt.Sprintf(format, hCode, hLabel, hTier, hDuration, hRedeemed, hCreated, hLastRedeemed)
	fmt.Fprint(out, header)
	fmt.Fprintln(out, strings.Repeat("-", len(strings.TrimRight(header, "\n"))))
	for _, r := range rows {
		fmt.Fprintf(out, format, r.code, r.label, r.tier, r.duration, r.redeemed, r.created, r.lastRedeemed)
	}
}
