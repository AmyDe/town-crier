using System.Diagnostics;
using TownCrier.Application.Observability;
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
    private readonly IWatchZoneRepository watchZoneRepository;
    private readonly INotificationEnqueuer notificationEnqueuer;

    public PollPlanItCommandHandler(
        IPlanItClient planItClient,
        IPollStateStore pollStateStore,
        IPlanningApplicationRepository applicationRepository,
        TimeProvider timeProvider,
        IActiveAuthorityProvider activeAuthorityProvider,
        IWatchZoneRepository watchZoneRepository,
        INotificationEnqueuer notificationEnqueuer)
    {
        this.planItClient = planItClient;
        this.pollStateStore = pollStateStore;
        this.applicationRepository = applicationRepository;
        this.timeProvider = timeProvider;
        this.activeAuthorityProvider = activeAuthorityProvider;
        this.watchZoneRepository = watchZoneRepository;
        this.notificationEnqueuer = notificationEnqueuer;
    }

    public async Task<PollPlanItResult> HandleAsync(PollPlanItCommand command, CancellationToken ct)
    {
        var lastPollTime = await this.pollStateStore.GetLastPollTimeAsync(ct).ConfigureAwait(false);
        var now = this.timeProvider.GetUtcNow();
        lastPollTime ??= now.AddDays(-30);
        var authorityIds = await this.activeAuthorityProvider.GetActiveAuthorityIdsAsync(ct).ConfigureAwait(false);

        var count = 0;
        var authoritiesPolled = 0;
        foreach (var authorityId in authorityIds)
        {
            using var authorityActivity = PollingInstrumentation.Source.StartActivity("Poll Authority");
            authorityActivity?.SetTag("polling.authority_code", authorityId);
            var authorityStart = Stopwatch.GetTimestamp();

            var authorityAppCount = 0;
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

                authorityAppCount++;
                count++;
            }

            PollingMetrics.AuthorityProcessingDuration.Record(Stopwatch.GetElapsedTime(authorityStart).TotalMilliseconds);
            PollingMetrics.ApplicationsIngested.Add(authorityAppCount);
            authorityActivity?.SetTag("polling.applications_found", authorityAppCount);

            PollingMetrics.AuthoritiesPolled.Add(1);
            await this.pollStateStore.SaveLastPollTimeAsync(now, ct).ConfigureAwait(false);
            authoritiesPolled++;
        }

        return new PollPlanItResult(count, authoritiesPolled);
    }
}
