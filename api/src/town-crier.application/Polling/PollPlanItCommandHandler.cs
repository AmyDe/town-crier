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
            var capHit = false;
            var lastPollTime = now.AddDays(-1);
            var lastPageFetched = 0;
            int? firstPageTotal = null;

            var hadActiveCursor = false;
            try
            {
                var existingState = await this.pollStateStore.GetAsync(authorityId, ct).ConfigureAwait(false);
                lastPollTime = existingState?.LastPollTime ?? now.AddDays(-1);
                hadActiveCursor = existingState?.Cursor is not null;

                // Resume from a previously-saved cursor only when its recorded date matches
                // the date we're about to query. If the HWM has advanced past the cursor's
                // date the cursor is stale and must be ignored (see tc-70kg / polling-resumable-cursor).
                // Overlap by one page (-1) to tolerate PlanIt page-shift between cycles.
                var startPage = 1;
                if (existingState?.Cursor is { } resumeCursor
                    && resumeCursor.DifferentStart == DateOnly.FromDateTime(lastPollTime.UtcDateTime))
                {
                    startPage = Math.Max(1, resumeCursor.NextPage - 1);
                }

                var maxPages = this.options.MaxPagesPerAuthorityPerCycle;
                var pagesFetched = 0;
                var page = startPage;
                while (true)
                {
                    var pageResult = await this.planItClient.FetchApplicationsPageAsync(
                        authorityId, lastPollTime, page, ct).ConfigureAwait(false);

                    if (pagesFetched == 0)
                    {
                        firstPageTotal = pageResult.Total;

                        // Emit authority_total gauge + span tag on the first page of this
                        // authority's fetch (see docs/specs/polling-resumable-cursor.md#telemetry-additions).
                        // PlanIt occasionally omits the total; skip emission in that case so
                        // downstream dashboards don't see spurious zeros.
                        if (pageResult.Total is { } totalValue)
                        {
                            PollingMetrics.AuthorityTotal.Record(
                                totalValue,
                                cycleTypeTag,
                                new KeyValuePair<string, object?>("polling.authority_code", authorityId));
                            authorityActivity?.SetTag("polling.authority_total", totalValue);
                        }
                    }

                    foreach (var application in pageResult.Applications)
                    {
                        authorityAppCount++;

                        if (application.LastDifferent > (highWaterMark ?? DateTimeOffset.MinValue))
                        {
                            highWaterMark = application.LastDifferent;
                        }

                        // Skip upsert + zone fan-out when the only change is PlanIt bookkeeping
                        // (LastDifferent bumped by a rescrape). This is the load-bearing fix for
                        // reindex floods — see bd tc-yt57. Partition-scoped lookup (authorityCode
                        // is the Applications container's partition key) avoids a cross-partition
                        // fan-out on the hot path — see bd tc-vidz.
                        var authorityCode = application.AreaId.ToString(System.Globalization.CultureInfo.InvariantCulture);
                        var existing = await this.applicationRepository.GetByUidAsync(application.Uid, authorityCode, ct).ConfigureAwait(false);
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
                    lastPageFetched = page;

                    if (!pageResult.HasMorePages)
                    {
                        break;
                    }

                    // Voluntary page cap — bail cleanly so seed-poll cycles can't burn a
                    // backlogged authority's full rate budget before rotating. Cap-hit
                    // freezes the HWM and persists a resumable cursor so the next cycle
                    // picks up where this one left off. See docs/specs/polling-resumable-cursor.md.
                    if (maxPages.HasValue && pagesFetched >= maxPages.Value)
                    {
                        capHit = true;
                        break;
                    }

                    page++;
                }

                completedSuccessfully = true;
            }
            catch (HttpRequestException ex) when (ex.StatusCode == HttpStatusCode.TooManyRequests)
            {
                // 429 is an expected, handled outcome — skip the authority, increment
                // rate_limited, and (elsewhere below) save a resumable cursor so the
                // next cycle picks up where this one left off. Do NOT call AddException
                // or SetStatus(Error) here: that surfaces a routine throttle event as an
                // unhandled exception in App Insights (see bd tc-qc65).
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

                // Cursor lifecycle — see docs/specs/polling-resumable-cursor.md.
                //   * Cap hit or 429 mid-pagination → freeze HWM, save cursor at next
                //     unfetched page so the following cycle resumes from there.
                //   * Natural end (or non-429 exception with partial progress) → advance
                //     HWM to max LastDifferent observed, clear any active cursor.
                if (capHit || (rateLimited && authorityAppCount > 0))
                {
                    var nextPage = lastPageFetched + 1;
                    await this.pollStateStore.SaveAsync(
                        authorityId,
                        lastPollTime,
                        new PollCursor(
                            DateOnly.FromDateTime(lastPollTime.UtcDateTime),
                            nextPage,
                            firstPageTotal),
                        ct).ConfigureAwait(false);
                    PollingMetrics.CursorAdvanced.Add(1, cycleTypeTag);
                    authorityActivity?.SetTag("polling.cursor.next_page", nextPage);
                }
                else
                {
                    await this.pollStateStore.SaveAsync(
                        authorityId, highWaterMark ?? now, cursor: null, ct).ConfigureAwait(false);

                    // Only emit cursor_cleared when we actually *cleared* something —
                    // i.e., there was a previously-active cursor. Clearing "nothing"
                    // would dilute the signal for alerting on stuck cursors.
                    if (hadActiveCursor)
                    {
                        PollingMetrics.CursorCleared.Add(1, cycleTypeTag);
                    }
                }

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
