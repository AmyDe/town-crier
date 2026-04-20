namespace TownCrier.Infrastructure.Polling;

internal sealed class PollStateDocument
{
    public required string Id { get; init; }

    public required string LastPollTime { get; init; }

    public required int AuthorityId { get; init; }

    // Cursor fields — all three move as a set. Absent when there is no active
    // resumable pagination cursor. See docs/specs/polling-resumable-cursor.md.
    public string? CursorDifferentStart { get; init; }

    public int? CursorNextPage { get; init; }

    public int? CursorKnownTotal { get; init; }
}
