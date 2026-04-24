namespace TownCrier.Infrastructure.Polling;

internal sealed class PollStateDocument
{
    public required string Id { get; init; }

    public required string LastPollTime { get; init; }

    public required int AuthorityId { get; init; }

    // Separate high-water mark — the max LastDifferent observed for this authority,
    // used as the PlanIt different_start cursor. Split from LastPollTime so quiet
    // authorities don't stall the scheduler. See docs/specs/poll-state-split-last-poll-time.md.
    // Nullable for backward compatibility with legacy documents written before the split —
    // CosmosPollStateStore falls back to LastPollTime when this is absent.
    public string? HighWaterMark { get; init; }

    // Cursor fields — all three move as a set. Absent when there is no active
    // resumable pagination cursor. See docs/specs/polling-resumable-cursor.md.
    public string? CursorDifferentStart { get; init; }

    public int? CursorNextPage { get; init; }

    public int? CursorKnownTotal { get; init; }
}
