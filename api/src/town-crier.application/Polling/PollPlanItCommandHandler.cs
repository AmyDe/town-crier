using TownCrier.Application.PlanIt;

namespace TownCrier.Application.Polling;

public sealed class PollPlanItCommandHandler
{
    private readonly IPlanItClient planItClient;
    private readonly IPollStateStore pollStateStore;
    private readonly TimeProvider timeProvider;

    public PollPlanItCommandHandler(
        IPlanItClient planItClient,
        IPollStateStore pollStateStore,
        TimeProvider timeProvider)
    {
        this.planItClient = planItClient;
        this.pollStateStore = pollStateStore;
        this.timeProvider = timeProvider;
    }

    public async Task<PollPlanItResult> HandleAsync(PollPlanItCommand command, CancellationToken ct)
    {
        var lastPollTime = await this.pollStateStore.GetLastPollTimeAsync(ct).ConfigureAwait(false);

        var count = 0;
        await foreach (var application in this.planItClient.FetchApplicationsAsync(lastPollTime, ct).ConfigureAwait(false))
        {
            count++;
        }

        await this.pollStateStore.SaveLastPollTimeAsync(this.timeProvider.GetUtcNow(), ct).ConfigureAwait(false);

        return new PollPlanItResult(count);
    }
}
