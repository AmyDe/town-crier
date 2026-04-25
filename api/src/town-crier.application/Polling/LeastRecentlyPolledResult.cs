namespace TownCrier.Application.Polling;

/// <summary>
/// Result of <see cref="IPollStateStore.GetLeastRecentlyPolledAsync"/>: the LRU-sorted
/// authority ids the cycle should walk, plus the count of authorities with no PollState
/// document at all (the "never-polled" cohort). Both pieces of information come from
/// the same in-memory pass over loaded PollState docs, so returning them together avoids
/// a duplicate Cosmos query when the handler needs the never-polled count for telemetry.
/// </summary>
/// <param name="AuthorityIds">
/// Authority ids ordered never-polled-first, then ascending <c>LastPollTime</c>.
/// </param>
/// <param name="NeverPolledCount">
/// Number of <see cref="AuthorityIds"/> entries that have no PollState document.
/// Drains monotonically toward 0 as the Seed cycle rotates through the cohort.
/// See bd tc-ews7 / tc-ifdl.
/// </param>
public sealed record LeastRecentlyPolledResult(
    IReadOnlyList<int> AuthorityIds,
    int NeverPolledCount);
