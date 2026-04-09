using System.Diagnostics;
using System.Net;
using Microsoft.Extensions.Logging;
using TownCrier.Application.Observability;
using TownCrier.Application.PlanIt;
using TownCrier.Application.PlanningApplications;
using TownCrier.Application.WatchZones;

namespace TownCrier.Application.Polling;

public sealed partial class PollPlanItCommandHandler
{
    private readonly IPlanItClient planItClient;
    private readonly IPollStateStore pollStateStore;
    private readonly IPlanningApplicationRepository applicationRepository;
    private readonly TimeProvider timeProvider;
    private readonly IActiveAuthorityProvider activeAuthorityProvider;
    private readonly IWatchZoneRepository watchZoneRepository;
    private readonly INotificationEnqueuer notificationEnqueuer;
    private readonly ILogger<PollPlanItCommandHandler> logger;

    public PollPlanItCommandHandler(
        IPlanItClient planItClient,
        IPollStateStore pollStateStore,
        IPlanningApplicationRepository applicationRepository,
        TimeProvider timeProvider,
        IActiveAuthorityProvider activeAuthorityProvider,
        IWatchZoneRepository watchZoneRepository,
        INotificationEnqueuer notificationEnqueuer,
        ILogger<PollPlanItCommandHandler> logger)
    {
        this.planItClient = planItClient;
        this.pollStateStore = pollStateStore;
        this.applicationRepository = applicationRepository;
        this.timeProvider = timeProvider;
        this.activeAuthorityProvider = activeAuthorityProvider;
        this.watchZoneRepository = watchZoneRepository;
        this.notificationEnqueuer = notificationEnqueuer;
        this.logger = logger;
    }

    public async Task<PollPlanItResult> HandleAsync(PollPlanItCommand command, CancellationToken ct)
    {
        var now = this.timeProvider.GetUtcNow();
        var activeIds = await this.activeAuthorityProvider.GetActiveAuthorityIdsAsync(ct).ConfigureAwait(false);
        var sortedIds = await this.pollStateStore.GetLeastRecentlyPolledAsync(
            activeIds.ToList(), ct).ConfigureAwait(false);

        var count = 0;
        var authoritiesPolled = 0;
        var rateLimited = false;
        foreach (var authorityId in sortedIds)
        {
            using var authorityActivity = PollingInstrumentation.Source.StartActivity("Poll Authority");
            authorityActivity?.SetTag("polling.authority_code", authorityId);
            var authorityStart = Stopwatch.GetTimestamp();

            try
            {
                var lastPollTime = await this.pollStateStore.GetLastPollTimeAsync(authorityId, ct).ConfigureAwait(false);
                lastPollTime ??= now.AddDays(-30);

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
                }

                PollingMetrics.AuthorityProcessingDuration.Record(Stopwatch.GetElapsedTime(authorityStart).TotalMilliseconds);
                authorityActivity?.SetTag("polling.applications_found", authorityAppCount);

                await this.pollStateStore.SaveLastPollTimeAsync(authorityId, now, ct).ConfigureAwait(false);
                PollingMetrics.AuthoritiesPolled.Add(1);
                PollingMetrics.ApplicationsIngested.Add(authorityAppCount);
                authoritiesPolled++;
                count += authorityAppCount;
            }
            catch (HttpRequestException ex) when (ex.StatusCode == HttpStatusCode.TooManyRequests)
            {
                PollingMetrics.AuthoritiesSkipped.Add(1);
                PollingMetrics.RateLimited.Add(1);
                PollingMetrics.AuthorityProcessingDuration.Record(Stopwatch.GetElapsedTime(authorityStart).TotalMilliseconds);
                rateLimited = true;
                LogRateLimitStop(this.logger, authorityId, ex);
                break;
            }
            catch (Exception ex) when (ex is not OperationCanceledException)
            {
                PollingMetrics.AuthoritiesSkipped.Add(1);
                PollingMetrics.AuthorityProcessingDuration.Record(Stopwatch.GetElapsedTime(authorityStart).TotalMilliseconds);
                LogAuthorityError(this.logger, authorityId, ex);
            }
        }

        await this.pollStateStore.DeleteGlobalPollStateAsync(ct).ConfigureAwait(false);

        return new PollPlanItResult(count, authoritiesPolled, rateLimited);
    }

    [LoggerMessage(Level = LogLevel.Warning, Message = "Rate limited polling authority {AuthorityId}, stopping polling cycle")]
    private static partial void LogRateLimitStop(ILogger logger, int authorityId, Exception exception);

    [LoggerMessage(Level = LogLevel.Error, Message = "Error polling authority {AuthorityId}, skipping to next authority")]
    private static partial void LogAuthorityError(ILogger logger, int authorityId, Exception exception);
}
