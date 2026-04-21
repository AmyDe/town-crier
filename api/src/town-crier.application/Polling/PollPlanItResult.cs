namespace TownCrier.Application.Polling;

/// <summary>
/// Result of a <see cref="PollPlanItCommandHandler"/> run.
/// </summary>
/// <param name="ApplicationCount">Total applications ingested (upserted) across all authorities.</param>
/// <param name="AuthoritiesPolled">Number of authorities that completed fetching (successfully or after a
/// mid-pagination 429).</param>
/// <param name="RateLimited">Whether the cycle stopped because PlanIt returned 429.</param>
/// <param name="TerminationReason">Why the cycle ended — see <see cref="PollTerminationReason"/>.</param>
/// <param name="AuthorityErrors">Count of per-authority non-rate-limit errors observed during the run.
/// Used by the worker to decide the exit code: if <c>ApplicationCount</c> is zero AND
/// <c>AuthorityErrors</c> is zero, the cycle had no useful work to do and exits 0.</param>
/// <param name="RetryAfter">Optional <c>Retry-After</c> hint bubbled up from PlanIt's 429 response when
/// <see cref="RateLimited"/> is <c>true</c>. Consumed by the next-run scheduler to decide when to publish
/// the next Service Bus trigger — capped and jittered by <see cref="PollNextRunScheduler"/>.</param>
public sealed record PollPlanItResult(
    int ApplicationCount,
    int AuthoritiesPolled,
    bool RateLimited,
    PollTerminationReason TerminationReason,
    int AuthorityErrors,
    TimeSpan? RetryAfter = null);
