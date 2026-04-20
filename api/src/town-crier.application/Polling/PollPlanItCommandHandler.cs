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
    private readonly ICycleSelector cycleSelector;
    private readonly PollingOptions options;
    private readonly ILogger<PollPlanItCommandHandler> logger;

    public PollPlanItCommandHandler(
        IPlanItClient planItClient,
        IPollStateStore pollStateStore,
        IPlanningApplicationRepository applicationRepository,
        TimeProvider timeProvider,
        IActiveAuthorityProvider activeAuthorityProvider,
        IWatchZoneRepository watchZoneRepository,
        INotificationEnqueuer notificationEnqueuer,
        ICycleSelector cycleSelector,
        PollingOptions options,
        ILogger<PollPlanItCommandHandler> logger)
    {
        this.planItClient = planItClient;
        this.pollStateStore = pollStateStore;
        this.applicationRepository = applicationRepository;
        this.timeProvider = timeProvider;
        this.activeAuthorityProvider = activeAuthorityProvider;
        this.watchZoneRepository = watchZoneRepository;
        this.notificationEnqueuer = notificationEnqueuer;
        this.cycleSelector = cycleSelector;
        this.options = options;
        this.logger = logger;
    }

    public async Task<PollPlanItResult> HandleAsync(PollPlanItCommand command, CancellationToken ct)
    {
        var now = this.timeProvider.GetUtcNow();
        var cycleType = this.cycleSelector.GetCurrent();
        var cycleTypeValue = cycleType.ToTelemetryValue();
        var cycleTypeTag = new KeyValuePair<string, object?>("cycle.type", cycleTypeValue);
        var activeIds = await this.activeAuthorityProvider.GetActiveAuthorityIdsAsync(ct).ConfigureAwait(false);
        var sortedIds = await this.pollStateStore.GetLeastRecentlyPolledAsync(
            activeIds.ToList(), ct).ConfigureAwait(false);

        var count = 0;
        var authoritiesPolled = 0;
        var rateLimited = false;
        var authorityErrors = 0;
        var timeBounded = false;
        foreach (var authorityId in sortedIds)
        {
            // Graceful timeout: when the worker's CTS (bounded to replicaTimeout - grace) fires,
            // break before starting a new authority so the partial result is returned cleanly.
            // See bd tc-qdtu — Container Apps replicaTimeout was SIGTERM-ing mid-loop and marking
            // otherwise-successful seed cycles as Failed.
            if (ct.IsCancellationRequested)
            {
                timeBounded = true;
                break;
            }

            using var authorityActivity = PollingInstrumentation.Source.StartActivity("Poll Authority");
            authorityActivity?.SetTag("polling.authority_code", authorityId);
            authorityActivity?.SetTag("cycle.type", cycleTypeValue);
            var authorityStart = Stopwatch.GetTimestamp();

            var authorityAppCount = 0;
            DateTimeOffset? highWaterMark = null;
            var completedSuccessfully = false;

            try
            {
                var existingState = await this.pollStateStore.GetAsync(authorityId, ct).ConfigureAwait(false);
                var lastPollTime = existingState?.LastPollTime ?? now.AddDays(-1);

                var maxPages = this.options.MaxPagesPerAuthorityPerCycle;
                var pagesFetched = 0;
                var page = 1;
                while (true)
                {
                    var pageResult = await this.planItClient.FetchApplicationsPageAsync(
                        authorityId, lastPollTime, page, ct).ConfigureAwait(false);

                    foreach (var application in pageResult.Applications)
                    {
                        authorityAppCount++;

                        if (application.LastDifferent > (highWaterMark ?? DateTimeOffset.MinValue))
                        {
                            highWaterMark = application.LastDifferent;
                        }

                        // Skip upsert + zone fan-out when the only change is PlanIt bookkeeping
                        // (LastDifferent bumped by a rescrape). This is the load-bearing fix for
                        // reindex floods — see bd tc-yt57.
                        var existing = await this.applicationRepository.GetByUidAsync(application.Uid, ct).ConfigureAwait(false);
                        if (existing is not null && existing.HasSameBusinessFieldsAs(application))
                        {
                            continue;
                        }

                        await this.applicationRepository.UpsertAsync(application, ct).ConfigureAwait(false);

                        if (application.Latitude.HasValue && application.Longitude.HasValue)
                        {
                            var matchingZones = await this.watchZoneRepository.FindZonesContainingAsync(
                                application.Latitude.Value, application.Longitude.Value, ct).ConfigureAwait(false);

                            foreach (var zone in matchingZones)
                            {
                                if (zone.CreatedAt > application.LastDifferent)
                                {
                                    continue;
                                }

                                await this.notificationEnqueuer.EnqueueAsync(application, zone, ct).ConfigureAwait(false);
                            }
                        }
                    }

                    pagesFetched++;

                    if (!pageResult.HasMorePages)
                    {
                        break;
                    }

                    // Voluntary page cap — bail cleanly so seed-poll cycles can't burn a
                    // backlogged authority's full rate budget before rotating. The cursor
                    // lifecycle replaces this in bd tc-70kg; until then the HWM alone drives
                    // resumption at the LastDifferent of the final streamed application.
                    if (maxPages.HasValue && pagesFetched >= maxPages.Value)
                    {
                        break;
                    }

                    page++;
                }

                completedSuccessfully = true;
            }
            catch (HttpRequestException ex) when (ex.StatusCode == HttpStatusCode.TooManyRequests)
            {
                authorityActivity?.AddException(ex);
                authorityActivity?.SetStatus(ActivityStatusCode.Error, ex.Message);
                PollingMetrics.RateLimited.Add(1, cycleTypeTag);
                rateLimited = true;
                LogRateLimitStop(this.logger, authorityId, ex);
            }
            catch (Exception ex) when (ex is not OperationCanceledException)
            {
                authorityActivity?.AddException(ex);
                authorityActivity?.SetStatus(ActivityStatusCode.Error, ex.Message);
                authorityErrors++;
                LogAuthorityError(this.logger, authorityId, ex);
            }

            PollingMetrics.AuthorityProcessingDuration.Record(Stopwatch.GetElapsedTime(authorityStart).TotalMilliseconds, cycleTypeTag);
            authorityActivity?.SetTag("polling.applications_found", authorityAppCount);

            if (completedSuccessfully || authorityAppCount > 0)
            {
                PollingMetrics.AuthoritiesPolled.Add(1, cycleTypeTag);
                PollingMetrics.ApplicationsIngested.Add(authorityAppCount, cycleTypeTag);
                await this.pollStateStore.SaveAsync(authorityId, highWaterMark ?? now, cursor: null, ct).ConfigureAwait(false);
                authoritiesPolled++;
                count += authorityAppCount;
            }
            else
            {
                PollingMetrics.AuthoritiesSkipped.Add(1, cycleTypeTag);
            }

            if (rateLimited)
            {
                break;
            }
        }

        PollTerminationReason terminationReason;
        if (rateLimited)
        {
            terminationReason = PollTerminationReason.RateLimited;
        }
        else if (timeBounded)
        {
            terminationReason = PollTerminationReason.TimeBounded;
        }
        else
        {
            terminationReason = PollTerminationReason.Natural;
        }

        PollingMetrics.CyclesCompleted.Add(
            1,
            cycleTypeTag,
            new KeyValuePair<string, object?>("termination", terminationReason.ToTelemetryValue()));

        return new PollPlanItResult(
            count,
            authoritiesPolled,
            rateLimited,
            terminationReason,
            authorityErrors);
    }

    [LoggerMessage(Level = LogLevel.Warning, Message = "Rate limited polling authority {AuthorityId}, stopping polling cycle")]
    private static partial void LogRateLimitStop(ILogger logger, int authorityId, Exception exception);

    [LoggerMessage(Level = LogLevel.Error, Message = "Error polling authority {AuthorityId}, skipping to next authority")]
    private static partial void LogAuthorityError(ILogger logger, int authorityId, Exception exception);
}
