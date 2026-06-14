package polling

import "time"

// PollCursor is a resumable PlanIt pagination cursor: where a previous cycle
// stopped mid-pagination so the next cycle resumes from the same different_start
// date and page. All three fields move as a set. Mirrors .NET PollCursor.
type PollCursor struct {
	// DifferentStart is the PlanIt different_start date the cursor was recorded
	// against. The cursor is valid only while the authority's high-water mark
	// still matches this date; once the HWM advances the cursor is stale.
	DifferentStart time.Time
	// NextPage is the next unfetched page number (1-based).
	NextPage int
	// KnownTotal is the total PlanIt reported on the first page of the cycle that
	// recorded the cursor, if known. Telemetry only. nil when unknown.
	KnownTotal *int
}

// PollState is the combined poll-state snapshot for one authority: when it was
// last polled (scheduling clock), its PlanIt high-water mark (cursoring), and an
// optional resumable pagination cursor. Mirrors .NET PollState.
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
// Mirrors .NET LeastRecentlyPolledResult.
type LeastRecentlyPolledResult struct {
	// AuthorityIDs are ordered never-polled-first, then ascending LastPollTime.
	AuthorityIDs []int
	// NeverPolledCount is the number of candidates with no PollState document.
	NeverPolledCount int
}
