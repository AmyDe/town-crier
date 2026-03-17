using TownCrier.Application.PlanIt;
using TownCrier.Application.PlanningApplications;
using TownCrier.Application.WatchZones;
using TownCrier.Domain.Polling;

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
    private readonly PollingScheduleConfig scheduleConfig;

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
        INotificationEnqueuer notificationEnqueuer,
        PollingScheduleConfig? scheduleConfig = null)
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
        this.scheduleConfig = scheduleConfig ?? new PollingScheduleConfig(HighThreshold: 5, LowThreshold: 2);
    }

    public async Task<PollPlanItResult> HandleAsync(PollPlanItCommand command, CancellationToken ct)
    {
        var lastPollTime = await this.pollStateStore.GetLastPollTimeAsync(ct).ConfigureAwait(false);
        var authorityIds = await this.activeAuthorityProvider.GetActiveAuthorityIdsAsync(ct).ConfigureAwait(false);
        var health = await this.pollingHealthStore.GetAsync(ct).ConfigureAwait(false);
        var now = this.timeProvider.GetUtcNow();

        var zoneCounts = await this.watchZoneRepository.GetZoneCountsByAuthorityAsync(ct).ConfigureAwait(false);
        var schedule = PollingSchedule.Calculate(zoneCounts, this.scheduleConfig);

        var authoritiesToPoll = authorityIds
            .Where(id => schedule.ShouldPollInCycle(id, command.CycleNumber))
            .ToList();

        var totalActive = authorityIds.Count;
        var polled = authoritiesToPoll.Count;
        var skipped = totalActive - polled;

        try
        {
            var count = 0;
            foreach (var authorityId in authoritiesToPoll)
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

            return new PollPlanItResult(count, polled, skipped, totalActive);
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
