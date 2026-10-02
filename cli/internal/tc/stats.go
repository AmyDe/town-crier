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
	if s.Paying.InTrial != nil {
		fmt.Fprintf(out, "  In trial: %d\n", *s.Paying.InTrial)
	}
	fmt.Fprintf(out, "  %s\n", estMRRLine(s.Paying))
	if s.Paying.AppStoreProAnnual != nil {
		fmt.Fprintf(out, "  %s\n", annualPlansLine(*s.Paying.AppStoreProAnnual))
	}
	if s.Paying.Lifetime != nil {
		fmt.Fprintf(out, "  Lifetime (App Store): %d\n", *s.Paying.Lifetime)
		fmt.Fprintf(out, "  %s\n", lifetimeRevenueLine(*s.Paying.Lifetime))
	}
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
// when the API predates the tier split needed to compute it. With annual Pro
// payers it also shows the monthly and annual components.
func estMRRLine(p statsPaying) string {
	if p.AppStoreByTier == nil {
		return "Est. MRR: -"
	}
	line := "Est. MRR: " + formatMRR(p)
	if p.AppStoreProAnnual != nil && *p.AppStoreProAnnual > 0 {
		line += fmt.Sprintf(" (monthly %s + annual %s)", formatPence(monthlyMRRPence(p)), formatPence(annualMRRPence(p)))
	}
	return line
}

// annualPlansLine renders the yearly cash value of annual Pro subscriptions.
func annualPlansLine(annual int) string {
	if annual == 0 {
		return "Annual plans: 0"
	}
	return fmt.Sprintf("Annual plans: %d × %s = %s/yr", annual, formatPence(proAnnualPence), formatPence(annual*proAnnualPence))
}

// lifetimeRevenueLine renders one-off lifetime revenue at list price. It is
// never part of MRR.
func lifetimeRevenueLine(lifetime int) string {
	if lifetime == 0 {
		return "Lifetime revenue: " + formatPence(0)
	}
	return fmt.Sprintf("Lifetime revenue: %s (%d × %s)", formatPence(lifetime*proLifetimePence), lifetime, formatPence(proLifetimePence))
}

// Per-plan price in pence, App Store-backed payers only. Comped (offer/admin)
// users never contribute to MRR.
const (
	proPence         = 499
	personalPence    = 199
	proAnnualPence   = 2999
	proLifetimePence = 6999
	monthsPerYear    = 12
)

// monthlyMRRPence is the MRR from Personal and monthly Pro payers.
func monthlyMRRPence(p statsPaying) int {
	t := p.AppStoreByTier
	if t == nil {
		return 0
	}
	return t.Personal*personalPence + (t.Pro-annualCount(p))*proPence
}

// annualMRRPence is the MRR from annual Pro payers: a twelfth of the yearly
// price, rounded to the nearest penny.
func annualMRRPence(p statsPaying) int {
	if p.AppStoreByTier == nil {
		return 0
	}
	return (annualCount(p)*proAnnualPence + monthsPerYear/2) / monthsPerYear
}

func annualCount(p statsPaying) int {
	if p.AppStoreProAnnual == nil {
		return 0
	}
	return *p.AppStoreProAnnual
}

// mrrPence computes the estimated MRR in integer pence. A nil tier split is
// zero.
func mrrPence(p statsPaying) int {
	return monthlyMRRPence(p) + annualMRRPence(p)
}

// formatPence renders integer pence as "£X.YY".
func formatPence(pence int) string {
	return fmt.Sprintf("£%d.%02d", pence/100, pence%100)
}

// formatMRR renders the integer-pence MRR as "£X.YY/mo".
func formatMRR(p statsPaying) string {
	return formatPence(mrrPence(p)) + "/mo"
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
	return fmt.Sprintf(" · lifetime %d (%s)", *p.Lifetime, formatPence(*p.Lifetime*proLifetimePence))
}

// trialSummarySegment renders the optional trial segment of statsSummaryLine,
// empty when the API predates free-trial tracking.
func trialSummarySegment(p statsPaying) string {
	if p.InTrial == nil {
		return ""
	}
	return fmt.Sprintf(" · trial %d", *p.InTrial)
}

// statsSummaryLine condenses the aggregate into a single line for the
// list-users first-page header. The headline paying figure is App Store only;
// offer/admin comps are reported separately, never bundled in.
func statsSummaryLine(s *statsResponse) string {
	return fmt.Sprintf(
		"%d users (Free %d, Personal %d, Pro %d) · paying %d%s · %s%s · comped %d · lapsed %d · new 24h %d · active 24h %d",
		s.Users.Total, s.Users.ByTier.Free, s.Users.ByTier.Personal, s.Users.ByTier.Pro,
		s.Paying.AppStore, trialSummarySegment(s.Paying), mrrSummarySegment(s.Paying), lifetimeSummarySegment(s.Paying),
		s.Paying.Comped, s.Paying.Lapsed,
		s.Signups.Last24h, s.Activity.Active24h,
	)
}
