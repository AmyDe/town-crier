using TownCrier.Application.PlanIt;
using TownCrier.Application.PlanningApplications;

namespace TownCrier.Application.Polling;

public sealed class PollPlanItCommandHandler
{
    private readonly IPlanItClient planItClient;
    private readonly IPollStateStore pollStateStore;
    private readonly IPlanningApplicationRepository applicationRepository;
    private readonly TimeProvider timeProvider;

    public PollPlanItCommandHandler(
        IPlanItClient planItClient,
        IPollStateStore pollStateStore,
        IPlanningApplicationRepository applicationRepository,
        TimeProvider timeProvider)
    {
        this.planItClient = planItClient;
        this.pollStateStore = pollStateStore;
        this.applicationRepository = applicationRepository;
        this.timeProvider = timeProvider;
    }

    public async Task<PollPlanItResult> HandleAsync(PollPlanItCommand command, CancellationToken ct)
    {
        var lastPollTime = await this.pollStateStore.GetLastPollTimeAsync(ct).ConfigureAwait(false);

        var count = 0;
        await foreach (var application in this.planItClient.FetchApplicationsAsync(lastPollTime, ct).ConfigureAwait(false))
        {
            await this.applicationRepository.UpsertAsync(application, ct).ConfigureAwait(false);
            count++;
        }

        await this.pollStateStore.SaveLastPollTimeAsync(this.timeProvider.GetUtcNow(), ct).ConfigureAwait(false);

        return new PollPlanItResult(count);
    }
}
