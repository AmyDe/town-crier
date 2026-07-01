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

// defaultPageSize is the list-users default when no page size is given.
const defaultPageSize = 20

// runListUsers implements `tc list-users`: page through /v1/admin/users,
// printing a table per page and prompting before fetching the next one.
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

// printUsersTable renders one page as an aligned table. Column widths are
// computed per page (rather than hard-coded) so any user-id length — a short
// Auth0 id or a ~49-char Apple id — renders without pushing the later columns
// out of alignment.
func printUsersTable(out io.Writer, page *listUsersResponse) {
	const (
		hUserID     = "UserId"
		hEmail      = "Email"
		hTier       = "Tier"
		hWatchZones = "WatchZones"
		hLastActive = "LastActive"
		hCreated    = "Created"
		hNotifs     = "Notifs"
	)

	type cells struct {
		userID, email, tier, watchZones, lastActive, created, notifs string
	}
	rows := make([]cells, 0, len(page.Items))
	for _, item := range page.Items {
		rows = append(rows, cells{
			userID:     item.UserID,
			email:      emailCell(item.Email),
			tier:       item.Tier,
			watchZones: watchZonesCell(item.WatchZoneCount),
			lastActive: datePart(item.LastActiveAt),
			created:    datePart(item.CreatedAt),
			notifs:     fmt.Sprintf("%d/%d", item.NotificationUnread, item.NotificationTotal),
		})
	}

	// Seed widths from the headers, then widen to the longest cell per column.
	wUserID, wEmail, wTier := len(hUserID), len(hEmail), len(hTier)
	wWatch, wLast, wCreated, wNotifs := len(hWatchZones), len(hLastActive), len(hCreated), len(hNotifs)
	for _, r := range rows {
		wUserID = max(wUserID, len(r.userID))
		wEmail = max(wEmail, len(r.email))
		wTier = max(wTier, len(r.tier))
		wWatch = max(wWatch, len(r.watchZones))
		wLast = max(wLast, len(r.lastActive))
		wCreated = max(wCreated, len(r.created))
		wNotifs = max(wNotifs, len(r.notifs))
	}

	format := fmt.Sprintf("%%-%ds  %%-%ds  %%-%ds  %%-%ds  %%-%ds  %%-%ds  %%-%ds\n",
		wUserID, wEmail, wTier, wWatch, wLast, wCreated, wNotifs)

	header := fmt.Sprintf(format, hUserID, hEmail, hTier, hWatchZones, hLastActive, hCreated, hNotifs)
	fmt.Fprint(out, header)
	fmt.Fprintln(out, strings.Repeat("-", len(strings.TrimRight(header, "\n"))))
	for _, r := range rows {
		fmt.Fprintf(out, format, r.userID, r.email, r.tier, r.watchZones, r.lastActive, r.created, r.notifs)
	}
}

// emailCell renders an optional email, falling back to "(none)" when absent.
func emailCell(email *string) string {
	if email == nil {
		return "(none)"
	}
	return *email
}

// watchZonesCell renders the watch-zone count, or "-" for a legacy profile that
// never had the counter (nil).
func watchZonesCell(n *int) string {
	if n == nil {
		return "-"
	}
	return strconv.Itoa(*n)
}

// datePart trims an RFC3339 timestamp to its date portion (YYYY-MM-DD) for
// compact display. A nil or empty value renders as "-"; a value shorter than a
// full date is passed through unchanged.
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
