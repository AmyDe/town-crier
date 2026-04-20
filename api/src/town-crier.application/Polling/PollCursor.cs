namespace TownCrier.Application.Polling;

/// <summary>
/// Resumable pagination cursor for PlanIt polling. Captures where a previous
/// cycle stopped mid-pagination so the next cycle can resume from the same
/// <c>different_start</c> date and page number. All three fields move as a set.
/// </summary>
/// <param name="DifferentStart">
/// The PlanIt <c>different_start</c> date the cursor was recorded against. The
/// cursor is only valid while the authority's high-water mark still matches
/// this date; once the HWM advances the cursor is stale and must be ignored.
/// </param>
/// <param name="NextPage">The next unfetched page number (1-based).</param>
/// <param name="KnownTotal">
/// Total results reported by PlanIt on the first page fetched in the cycle
/// that recorded the cursor, if known. Used for telemetry / progress tracking.
/// </param>
public sealed record PollCursor(DateOnly DifferentStart, int NextPage, int? KnownTotal);
