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
	fmt.Fprintf(out, "  %s\n", payingAppStoreLine(s.Paying))
	if s.Paying.Lifetime != nil {
		fmt.Fprintf(out, "  Lifetime (App Store): %d\n", *s.Paying.Lifetime)
	}
	fmt.Fprintf(out, "  %s\n", estMRRLine(s.Paying))
	fmt.Fprintf(out, "  Comped (offer/admin): %d\n", s.Paying.Comped)
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

// payingAppStoreLine renders the App Store-only paying headline. A nil
// AppStoreByTier means an API build that predates the tier split, so it
// degrades to the bare count rather than guess a breakdown.
func payingAppStoreLine(p statsPaying) string {
	if p.AppStoreByTier == nil {
		return fmt.Sprintf("Paying (App Store): %d", p.AppStore)
	}
	line := fmt.Sprintf("Paying (App Store): %d (Personal %d, Pro %d",
		p.AppStore, p.AppStoreByTier.Personal, p.AppStoreByTier.Pro)
	if p.AppStoreProAnnual != nil {
		line += fmt.Sprintf(", of which %d annual", *p.AppStoreProAnnual)
	}
	return line + ")"
}

// estMRRLine renders the estimated monthly recurring revenue line, or "-"
// when the API predates the tier split needed to compute it.
func estMRRLine(p statsPaying) string {
	if p.AppStoreByTier == nil {
		return "Est. MRR: -"
	}
	return fmt.Sprintf("Est. MRR: %s", formatMRR(p))
}

// Per-plan price in pence, App Store-backed payers only. Comped (offer/admin)
// users never contribute to MRR.
const (
	proPence       = 499
	personalPence  = 199
	proAnnualPence = 2999
	monthsPerYear  = 12
)

// mrrPence computes the estimated MRR in integer pence. Annual Pro payers
// count at a twelfth of the yearly price, rounded to the nearest penny; the
// remaining Pro payers count at the monthly price. A nil tier split is zero.
func mrrPence(p statsPaying) int {
	t := p.AppStoreByTier
	if t == nil {
		return 0
	}
	annual := 0
	if p.AppStoreProAnnual != nil {
		annual = *p.AppStoreProAnnual
	}
	return t.Personal*personalPence + (t.Pro-annual)*proPence + (annual*proAnnualPence+monthsPerYear/2)/monthsPerYear
}

// formatMRR renders the integer-pence MRR as "£X.YY/mo".
func formatMRR(p statsPaying) string {
	pence := mrrPence(p)
	return fmt.Sprintf("£%d.%02d/mo", pence/100, pence%100)
}

// mrrSummarySegment renders the MRR segment of statsSummaryLine, degrading to
// "MRR -" when the API predates the tier split.
func mrrSummarySegment(p statsPaying) string {
	if p.AppStoreByTier == nil {
		return "MRR -"
	}
	return "MRR " + formatMRR(p)
}

// lifetimeSummarySegment renders the optional lifetime segment of
// statsSummaryLine, empty when the API predates lifetime Pro.
func lifetimeSummarySegment(p statsPaying) string {
	if p.Lifetime == nil {
		return ""
	}
	return fmt.Sprintf(" · lifetime %d", *p.Lifetime)
}

// statsSummaryLine condenses the aggregate into a single line for the
// list-users first-page header. The headline paying figure is App Store only;
// offer/admin comps are reported separately, never bundled in.
func statsSummaryLine(s *statsResponse) string {
	return fmt.Sprintf(
		"%d users (Free %d, Personal %d, Pro %d) · paying %d · %s%s · comped %d · lapsed %d · new 24h %d · active 24h %d",
		s.Users.Total, s.Users.ByTier.Free, s.Users.ByTier.Personal, s.Users.ByTier.Pro,
		s.Paying.AppStore, mrrSummarySegment(s.Paying), lifetimeSummarySegment(s.Paying),
		s.Paying.Comped, s.Paying.Lapsed,
		s.Signups.Last24h, s.Activity.Active24h,
	)
}
