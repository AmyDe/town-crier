namespace TownCrier.Application.Polling;

/// <summary>
/// Combined poll-state snapshot for a single authority: when it was last polled
/// (for scheduling), its PlanIt high-water mark (for cursoring), and (optionally)
/// a resumable pagination cursor. The fields are read and written together so
/// the "these move as a set" invariant is encoded in the type system.
/// </summary>
/// <param name="LastPollTime">
/// Wall-clock time of the last poll attempt for this authority. Drives the
/// <c>GetLeastRecentlyPolledAsync</c> ordering so quiet authorities drop to
/// the back of the queue immediately after being polled — independent of
/// whether PlanIt returned anything new. See docs/specs/poll-state-split-last-poll-time.md.
/// </param>
/// <param name="HighWaterMark">
/// Latest <c>LastDifferent</c> timestamp observed for applications belonging to
/// this authority. Used as the PlanIt <c>different_start</c> cursor on the next
/// fetch, and as the anchor date for <see cref="PollCursor.DifferentStart"/>.
/// </param>
/// <param name="Cursor">
/// Active pagination cursor, or <c>null</c> when no cycle is mid-pagination
/// against the current <see cref="HighWaterMark"/> date.
/// </param>
public sealed record PollState(
    DateTimeOffset LastPollTime,
    DateTimeOffset HighWaterMark,
    PollCursor? Cursor);
