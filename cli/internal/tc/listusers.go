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

const defaultPageSize = 20

func runListUsers(ctx context.Context, client *Client, env Env, args *ParsedArgs) int {
	search, hasSearch := args.GetOptional("search")

	pageSize := defaultPageSize
	if pageSizeStr, ok := args.GetOptional("page-size"); ok {
		n, valid := parseStrictInt(pageSizeStr)
		if !valid || n <= 0 {
			fmt.Fprintln(env.Err, "Invalid --page-size: must be a positive integer")
			return exitUsage
		}
		pageSize = n
	}

	reader := bufio.NewReader(env.In)
	continuationToken := ""
	firstPage := true

	for {
		path := buildListUsersPath(hasSearch, search, pageSize, continuationToken)

		var page *listUsersResponse
		if err := client.GetJSON(ctx, path, &page); err != nil {
			fmt.Fprintln(env.Err, err.Error())
			return exitRuntime
		}
		if page == nil {
			fmt.Fprintln(env.Err, "Empty response from API")
			return exitRuntime
		}

		if firstPage {
			printStatsSummary(ctx, client, env.Out)
			firstPage = false
		}
		printUsersTable(env.Out, page)

		if page.ContinuationToken == nil {
			break
		}
		continuationToken = *page.ContinuationToken

		fmt.Fprint(env.Out, "Next page? [y/N] ")
		line, _ := reader.ReadString('\n')
		if !strings.EqualFold(strings.TrimSpace(line), "y") {
			break
		}
	}

	return exitOK
}

// printStatsSummary swallows any failure (network, non-2xx, empty body) so the
// users table still renders; the summary is a convenience, not the payload.
func printStatsSummary(ctx context.Context, client *Client, out io.Writer) {
	var resp *statsResponse
	if err := client.GetJSON(ctx, "/v1/admin/stats", &resp); err != nil || resp == nil {
		return
	}
	fmt.Fprintln(out, statsSummaryLine(resp))
}

func buildListUsersPath(hasSearch bool, search string, pageSize int, continuationToken string) string {
	var sb strings.Builder
	sb.WriteString("/v1/admin/users?pageSize=")
	sb.WriteString(strconv.Itoa(pageSize))
	if hasSearch {
		sb.WriteString("&search=")
		sb.WriteString(url.QueryEscape(search))
	}
	if continuationToken != "" {
		sb.WriteString("&continuationToken=")
		sb.WriteString(url.QueryEscape(continuationToken))
	}
	return sb.String()
}

// Column widths are computed per page so a ~49-char Apple user id does not push
// later columns out of alignment.
func printUsersTable(out io.Writer, page *listUsersResponse) {
	const (
		hUserID     = "UserId"
		hEmail      = "Email"
		hTier       = "Tier"
		hOffer      = "Offer"
		hWatchZones = "WatchZones"
		hLastActive = "LastActive"
		hCreated    = "Created"
		hNotifs     = "Notifs"
		hSaved      = "Saved"
		hDevices    = "Devices"
	)

	type cells struct {
		userID, email, tier, offer, watchZones, lastActive, created, notifs, saved, devices string
	}
	rows := make([]cells, 0, len(page.Items))
	for _, item := range page.Items {
		rows = append(rows, cells{
			userID:     item.UserID,
			email:      emailCell(item.Email),
			tier:       item.Tier,
			offer:      offerCell(item.OfferCode),
			watchZones: watchZonesCell(item.WatchZoneCount),
			lastActive: datePart(item.LastActiveAt),
			created:    datePart(item.CreatedAt),
			notifs:     fmt.Sprintf("%d/%d", item.NotificationUnread, item.NotificationTotal),
			saved:      strconv.Itoa(item.SavedCount),
			devices:    strconv.Itoa(item.DeviceCount),
		})
	}

	wUserID, wEmail, wTier, wOffer := len(hUserID), len(hEmail), len(hTier), len(hOffer)
	wWatch, wLast, wCreated, wNotifs := len(hWatchZones), len(hLastActive), len(hCreated), len(hNotifs)
	wSaved, wDevices := len(hSaved), len(hDevices)
	for _, r := range rows {
		wUserID = max(wUserID, len(r.userID))
		wEmail = max(wEmail, len(r.email))
		wTier = max(wTier, len(r.tier))
		wOffer = max(wOffer, len(r.offer))
		wWatch = max(wWatch, len(r.watchZones))
		wLast = max(wLast, len(r.lastActive))
		wCreated = max(wCreated, len(r.created))
		wNotifs = max(wNotifs, len(r.notifs))
		wSaved = max(wSaved, len(r.saved))
		wDevices = max(wDevices, len(r.devices))
	}

	format := fmt.Sprintf("%%-%ds  %%-%ds  %%-%ds  %%-%ds  %%-%ds  %%-%ds  %%-%ds  %%-%ds  %%-%ds  %%-%ds\n",
		wUserID, wEmail, wTier, wOffer, wWatch, wLast, wCreated, wNotifs, wSaved, wDevices)

	header := fmt.Sprintf(format, hUserID, hEmail, hTier, hOffer, hWatchZones, hLastActive, hCreated, hNotifs, hSaved, hDevices)
	fmt.Fprint(out, header)
	fmt.Fprintln(out, strings.Repeat("-", len(strings.TrimRight(header, "\n"))))
	for _, r := range rows {
		fmt.Fprintf(out, format, r.userID, r.email, r.tier, r.offer, r.watchZones, r.lastActive, r.created, r.notifs, r.saved, r.devices)
	}
}

func emailCell(email *string) string {
	if email == nil {
		return "(none)"
	}
	return *email
}

// watchZonesCell: nil means a legacy profile that never had the counter.
func watchZonesCell(n *int) string {
	if n == nil {
		return "-"
	}
	return strconv.Itoa(*n)
}

// offerCell: the API sends null for no active code, and for an erased user whose
// redeemed_by_user_id was scrubbed.
func offerCell(code *string) string {
	if code == nil {
		return "-"
	}
	return *code
}

func datePart(s *string) string {
	if s == nil || *s == "" {
		return "-"
	}
	v := *s
	if len(v) >= len("2006-01-02") {
		return v[:len("2006-01-02")]
	}
	return v
}
