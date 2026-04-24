using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

/// <summary>
/// Minimal <see cref="IPollPlanItCommandHandler"/> spy for orchestrator tests.
/// Records call count; can be configured to throw or to return a specific
/// termination reason.
/// </summary>
internal sealed class SpyHandler : IPollPlanItCommandHandler
{
    private int handleCalls;

    /// <summary>Gets the number of times <see cref="HandleAsync"/> was called.</summary>
    public int HandleCalls => this.handleCalls;

    /// <summary>
    /// Gets or sets the termination reason returned by the next <see cref="HandleAsync"/>
    /// call. Defaults to <see cref="PollTerminationReason.Natural"/>.
    /// </summary>
    public PollTerminationReason NextTerminationReason { get; set; } = PollTerminationReason.Natural;

    /// <summary>
    /// Gets or sets an exception to throw from <see cref="HandleAsync"/>. When
    /// set, the call count is still incremented before throwing.
    /// </summary>
    public Exception? ThrowsOnHandle { get; set; }

    public Task<PollPlanItResult> HandleAsync(PollPlanItCommand command, CancellationToken ct)
    {
        Interlocked.Increment(ref this.handleCalls);

        if (this.ThrowsOnHandle is { } ex)
        {
            throw ex;
        }

        var result = new PollPlanItResult(
            ApplicationCount: 0,
            AuthoritiesPolled: 1,
            RateLimited: false,
            TerminationReason: this.NextTerminationReason,
            AuthorityErrors: 0);

        return Task.FromResult(result);
    }
}
