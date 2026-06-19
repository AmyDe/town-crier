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

// defaultPageSize mirrors the .NET list-users default (pageSize ?? 20).
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

func printUsersTable(out io.Writer, page *listUsersResponse) {
	fmt.Fprintf(out, "%-24s %-32s %-10s\n", "UserId", "Email", "Tier")
	fmt.Fprintln(out, strings.Repeat("-", 66))
	for _, item := range page.Items {
		email := "(none)"
		if item.Email != nil {
			email = *item.Email
		}
		fmt.Fprintf(out, "%-24s %-32s %-10s\n", item.UserID, email, item.Tier)
	}
}
