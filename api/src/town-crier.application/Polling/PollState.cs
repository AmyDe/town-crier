namespace TownCrier.Application.Polling;

/// <summary>
/// Combined poll-state snapshot for a single authority: its current high-water
/// mark and (optionally) a resumable pagination cursor. The two fields are
/// read and written together so the "cursor fields move as a set" invariant
/// is encoded in the type system.
/// </summary>
/// <param name="LastPollTime">
/// High-water mark — the latest <c>LastDifferent</c> timestamp we have
/// observed for applications belonging to the authority.
/// </param>
/// <param name="Cursor">
/// Active pagination cursor, or <c>null</c> when no cycle is mid-pagination
/// against the current <see cref="LastPollTime"/> date.
/// </param>
public sealed record PollState(DateTimeOffset LastPollTime, PollCursor? Cursor);
