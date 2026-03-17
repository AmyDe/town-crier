using TownCrier.Application.PlanIt;
using TownCrier.Application.PlanningApplications;
using TownCrier.Application.WatchZones;

namespace TownCrier.Application.Polling;

public sealed class PollPlanItCommandHandler
{
    private readonly IPlanItClient planItClient;
    private readonly IPollStateStore pollStateStore;
    private readonly IPlanningApplicationRepository applicationRepository;
    private readonly TimeProvider timeProvider;
    private readonly IActiveAuthorityProvider activeAuthorityProvider;
    private readonly IPollingHealthStore pollingHealthStore;
    private readonly IPollingHealthAlerter pollingHealthAlerter;
    private readonly PollingHealthConfig healthConfig;
    private readonly IWatchZoneRepository watchZoneRepository;
    private readonly INotificationEnqueuer notificationEnqueuer;

    public PollPlanItCommandHandler(
        IPlanItClient planItClient,
        IPollStateStore pollStateStore,
        IPlanningApplicationRepository applicationRepository,
        TimeProvider timeProvider,
        IActiveAuthorityProvider activeAuthorityProvider,
        IPollingHealthStore pollingHealthStore,
        IPollingHealthAlerter pollingHealthAlerter,
        PollingHealthConfig healthConfig,
        IWatchZoneRepository watchZoneRepository,
        INotificationEnqueuer notificationEnqueuer)
    {
        this.planItClient = planItClient;
        this.pollStateStore = pollStateStore;
        this.applicationRepository = applicationRepository;
        this.timeProvider = timeProvider;
        this.activeAuthorityProvider = activeAuthorityProvider;
        this.pollingHealthStore = pollingHealthStore;
        this.pollingHealthAlerter = pollingHealthAlerter;
        this.healthConfig = healthConfig;
        this.watchZoneRepository = watchZoneRepository;
        this.notificationEnqueuer = notificationEnqueuer;
    }

    public async Task<PollPlanItResult> HandleAsync(PollPlanItCommand command, CancellationToken ct)
    {
        var lastPollTime = await this.pollStateStore.GetLastPollTimeAsync(ct).ConfigureAwait(false);
        var authorityIds = await this.activeAuthorityProvider.GetActiveAuthorityIdsAsync(ct).ConfigureAwait(false);
        var health = await this.pollingHealthStore.GetAsync(ct).ConfigureAwait(false);
        var now = this.timeProvider.GetUtcNow();

        try
        {
            var count = 0;
            foreach (var authorityId in authorityIds)
            {
                await foreach (var application in this.planItClient.FetchApplicationsAsync(authorityId, lastPollTime, ct).ConfigureAwait(false))
                {
                    await this.applicationRepository.UpsertAsync(application, ct).ConfigureAwait(false);

                    if (application.Latitude.HasValue && application.Longitude.HasValue)
                    {
                        var matchingZones = await this.watchZoneRepository.FindZonesContainingAsync(
                            application.Latitude.Value, application.Longitude.Value, ct).ConfigureAwait(false);

                        foreach (var zone in matchingZones)
                        {
                            await this.notificationEnqueuer.EnqueueAsync(application, zone, ct).ConfigureAwait(false);
                        }
                    }

                    count++;
                }
            }

            health.RecordSuccess(now);
            await this.pollingHealthStore.SaveAsync(health, ct).ConfigureAwait(false);
            await this.pollStateStore.SaveLastPollTimeAsync(now, ct).ConfigureAwait(false);

            return new PollPlanItResult(count);
        }
        catch
        {
            health.RecordFailure();
            await this.pollingHealthStore.SaveAsync(health, ct).ConfigureAwait(false);

            if (health.HasExceededFailureThreshold(this.healthConfig.MaxConsecutiveFailures))
            {
                await this.pollingHealthAlerter.AlertConsecutiveFailuresAsync(
                    health.ConsecutiveFailureCount, ct).ConfigureAwait(false);
            }

            if (health.IsStale(now, this.healthConfig.StalenessThreshold))
            {
                await this.pollingHealthAlerter.AlertStalenessAsync(
                    health.LastSuccessfulPollTime!.Value,
                    now - health.LastSuccessfulPollTime.Value,
                    ct).ConfigureAwait(false);
            }

            throw;
        }
    }
}
