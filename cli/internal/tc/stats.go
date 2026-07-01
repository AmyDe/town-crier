package tc

import (
	"context"
	"fmt"
	"io"
)

// runStats implements `tc stats`: fetch the whole-user-base aggregate from
// GET /v1/admin/stats and render it as a compact, grouped plain-text block.
// Error handling mirrors runListUsers: a non-2xx status or empty body prints to
// stderr and returns the runtime exit code.
func runStats(ctx context.Context, client *Client, env Env, _ *ParsedArgs) int {
	var resp *statsResponse
	if err := client.GetJSON(ctx, "/v1/admin/stats", &resp); err != nil {
		fmt.Fprintln(env.Err, err.Error())
		return exitRuntime
	}
	if resp == nil {
		fmt.Fprintln(env.Err, "Empty response from API")
		return exitRuntime
	}

	renderStats(env.Out, resp)
	return exitOK
}

// renderStats writes the aggregate as five labelled groups (Users, Paying,
// Signups, Activity, Reach). It is deliberately plain admin-tooling text — no
// product voice — kept compact enough to scan in a terminal.
func renderStats(out io.Writer, s *statsResponse) {
	fmt.Fprintln(out, "Users")
	fmt.Fprintf(out, "  Total: %d\n", s.Users.Total)
	fmt.Fprintf(out, "  By tier: Free %d, Personal %d, Pro %d\n",
		s.Users.ByTier.Free, s.Users.ByTier.Personal, s.Users.ByTier.Pro)
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Paying")
	fmt.Fprintf(out, "  Effective paid: %d\n", s.Paying.EffectivePaid)
	fmt.Fprintf(out, "  App Store: %d\n", s.Paying.AppStore)
	fmt.Fprintf(out, "  Comped: %d\n", s.Paying.Comped)
	fmt.Fprintf(out, "  Lapsed: %d\n", s.Paying.Lapsed)
	fmt.Fprintf(out, "  In grace: %d\n", s.Paying.InGrace)
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Signups")
	fmt.Fprintf(out, "  Last 24h: %d\n", s.Signups.Last24h)
	fmt.Fprintf(out, "  Last 7d: %d\n", s.Signups.Last7d)
	fmt.Fprintf(out, "  Last 30d: %d\n", s.Signups.Last30d)
	fmt.Fprintf(out, "  Most recent: %s\n", mostRecentCell(s.Signups.MostRecent))
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Activity")
	fmt.Fprintf(out, "  Active 24h: %d\n", s.Activity.Active24h)
	fmt.Fprintf(out, "  Active 7d: %d\n", s.Activity.Active7d)
	fmt.Fprintf(out, "  Zero watch zones: %d\n", s.Activity.ZeroWatchZones)
	fmt.Fprintf(out, "  No email: %d\n", s.Activity.NoEmail)
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Reach")
	fmt.Fprintf(out, "  Watch zones: %d\n", s.Reach.WatchZones)
	fmt.Fprintf(out, "  Saved applications: %d\n", s.Reach.SavedApplications)
	fmt.Fprintf(out, "  Device registrations: %d\n", s.Reach.DeviceRegistrations)
	fmt.Fprintf(out, "  Notifications sent: %d\n", s.Reach.NotificationsSent)
	fmt.Fprintf(out, "  Notifications unread: %d\n", s.Reach.NotificationsUnread)
}

// mostRecentCell renders the most-recent signup, degrading gracefully: "(none)"
// for an empty user base (nil) and "(none)" for a withheld email.
func mostRecentCell(mr *statsMostRecent) string {
	if mr == nil {
		return "(none)"
	}
	email := "none"
	if mr.Email != nil {
		email = *mr.Email
	}
	return fmt.Sprintf("%s (%s) at %s", mr.UserID, email, mr.CreatedAt)
}

// statsSummaryLine condenses the aggregate into a single line for the
// list-users first-page header: total + tier split, paying breakdown, and the
// two freshest signals (new-in-24h, active-in-24h).
func statsSummaryLine(s *statsResponse) string {
	return fmt.Sprintf(
		"%d users (Free %d, Personal %d, Pro %d) · paying %d (App Store %d, comped %d, lapsed %d) · new 24h %d · active 24h %d",
		s.Users.Total, s.Users.ByTier.Free, s.Users.ByTier.Personal, s.Users.ByTier.Pro,
		s.Paying.EffectivePaid, s.Paying.AppStore, s.Paying.Comped, s.Paying.Lapsed,
		s.Signups.Last24h, s.Activity.Active24h,
	)
}
