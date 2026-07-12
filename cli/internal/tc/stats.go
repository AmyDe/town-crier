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

// payingAppStoreLine renders the App Store-only paying headline — the
// "effective paid" figure (which bundles offer/admin comps) never appears
// here — with the Personal/Pro tier split when the API supplies it. A nil
// AppStoreByTier means an older API build that predates the split: degrade to
// the bare count rather than guess a breakdown.
func payingAppStoreLine(p statsPaying) string {
	if p.AppStoreByTier == nil {
		return fmt.Sprintf("Paying (App Store): %d", p.AppStore)
	}
	return fmt.Sprintf("Paying (App Store): %d (Personal %d, Pro %d)",
		p.AppStore, p.AppStoreByTier.Personal, p.AppStoreByTier.Pro)
}

// estMRRLine renders the estimated monthly recurring revenue line, or "-"
// when the API predates the tier split needed to compute it.
func estMRRLine(p statsPaying) string {
	if p.AppStoreByTier == nil {
		return "Est. MRR: -"
	}
	return fmt.Sprintf("Est. MRR: %s", formatMRR(p.AppStoreByTier))
}

// Per-tier monthly price in pence, App Store-backed payers only. Comped
// (offer/admin) users never contribute to MRR.
const (
	proPence      = 499
	personalPence = 199
)

// mrrPence computes the estimated MRR in integer pence — no floats anywhere
// near money. A nil tier (API predates the split) is zero, not an error: the
// caller decides whether to render that as "-" or "£0.00/mo".
func mrrPence(t *statsAppStoreByTier) int {
	if t == nil {
		return 0
	}
	return t.Pro*proPence + t.Personal*personalPence
}

// formatMRR renders the tier split's integer-pence MRR as "£X.YY/mo".
func formatMRR(t *statsAppStoreByTier) string {
	pence := mrrPence(t)
	return fmt.Sprintf("£%d.%02d/mo", pence/100, pence%100)
}

// mrrSummarySegment renders the MRR segment of statsSummaryLine, degrading to
// "MRR -" when the API predates the tier split.
func mrrSummarySegment(p statsPaying) string {
	if p.AppStoreByTier == nil {
		return "MRR -"
	}
	return "MRR " + formatMRR(p.AppStoreByTier)
}

// statsSummaryLine condenses the aggregate into a single line for the
// list-users first-page header: total + tier split, the App Store-only paying
// headline with estimated MRR, comped/lapsed, and the two freshest signals
// (new-in-24h, active-in-24h). The headline paying figure is App Store
// only — offer/admin comps are reported separately, never bundled in.
func statsSummaryLine(s *statsResponse) string {
	return fmt.Sprintf(
		"%d users (Free %d, Personal %d, Pro %d) · paying %d · %s · comped %d · lapsed %d · new 24h %d · active 24h %d",
		s.Users.Total, s.Users.ByTier.Free, s.Users.ByTier.Personal, s.Users.ByTier.Pro,
		s.Paying.AppStore, mrrSummarySegment(s.Paying), s.Paying.Comped, s.Paying.Lapsed,
		s.Signups.Last24h, s.Activity.Active24h,
	)
}
