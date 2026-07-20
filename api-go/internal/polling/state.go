package polling

import "time"

// PollCursor is a resumable PlanIt pagination cursor: where a previous cycle
// stopped mid-pagination so the next cycle resumes from the same different_start
// date and page. All three fields move as a set.
type PollCursor struct {
	// DifferentStart is the PlanIt different_start date the cursor was recorded
	// against. The cursor is valid only while the authority's high-water mark
	// still matches this date; once the HWM advances the cursor is stale.
	DifferentStart time.Time
	// NextIndex is the next unfetched record's 0-based offset (PlanIt's index=
	// parameter). Record-level resume is immune to PlanIt's 1MB response-body
	// truncation: a truncated fetch still advances NextIndex by the records
	// actually received, so a resume never skips or re-derives a page boundary
	// (GH#955). Replaces the old page-granular NextPage.
	NextIndex int
	// KnownTotal is the total PlanIt reported on the first page of the cycle that
	// recorded the cursor, if known. Telemetry only. nil when unknown.
	KnownTotal *int
	// WalkHead is a national lane's descending-walk true maximum LastDifferent
	// (ADR 0044 / GH#983), captured from the first record of the walk's first
	// page (index 0) and carried unchanged through every resume until the
	// walk completes. The zero value means unset -- no walk-head captured yet,
	// or a pre-migration legacy cursor -- same zero-time convention as
	// HighWaterMark elsewhere in this package. Unused by Lane C's ascending
	// epoch walk (lanec.go) and the legacy per-authority drain (handler.go),
	// which never set or read it.
	WalkHead time.Time
}

// PollState is the combined poll-state snapshot for one authority: when it was
// last polled (scheduling clock), its PlanIt high-water mark (cursoring), and an
// optional resumable pagination cursor.
type PollState struct {
	// LastPollTime is the wall-clock time of the last poll attempt. Drives the
	// least-recently-polled ordering so quiet authorities drop to the back of the
	// queue immediately, independent of whether PlanIt returned anything new.
	LastPollTime time.Time
	// HighWaterMark is the latest LastDifferent observed for this authority, used
	// as the PlanIt different_start cursor on the next fetch.
	HighWaterMark time.Time
	// Cursor is the active pagination cursor, or nil when no cycle is
	// mid-pagination against the current HighWaterMark date.
	Cursor *PollCursor
}

// LeastRecentlyPolledResult is the LRU-sorted candidate authority ids the cycle
// should walk, plus the count of never-polled candidates (no PollState doc).
type LeastRecentlyPolledResult struct {
	// AuthorityIDs are ordered never-polled-first, then ascending LastPollTime.
	AuthorityIDs []int
	// NeverPolledCount is the number of candidates with no PollState document.
	NeverPolledCount int
}
