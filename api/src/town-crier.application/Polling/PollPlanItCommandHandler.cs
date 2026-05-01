using System.Diagnostics;
using System.Net;
using Microsoft.Extensions.Logging;
using TownCrier.Application.Notifications;
using TownCrier.Application.Observability;
using TownCrier.Application.PlanIt;
using TownCrier.Application.PlanningApplications;
using TownCrier.Application.WatchZones;

namespace TownCrier.Application.Polling;

public sealed partial class PollPlanItCommandHandler : IPollPlanItCommandHandler
{
    private readonly IPlanItClient planItClient;
    private readonly IPollStateStore pollStateStore;
    private readonly IPlanningApplicationRepository applicationRepository;
    private readonly TimeProvider timeProvider;
    private readonly IActiveAuthorityProvider activeAuthorityProvider;
    private readonly IWatchZoneRepository watchZoneRepository;
    private readonly INotificationEnqueuer notificationEnqueuer;
    private readonly IDecisionEventDispatcher decisionEventDispatcher;
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
        IDecisionEventDispatcher decisionEventDispatcher,
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
        this.decisionEventDispatcher = decisionEventDispatcher;
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
        var deadline = this.options.HandlerBudget is { } budget
            ? (DateTimeOffset?)(now + budget)
            : null;
        var activeIds = await this.activeAuthorityProvider.GetActiveAuthorityIdsAsync(ct).ConfigureAwait(false);
        var leastRecentlyPolled = await this.pollStateStore.GetLeastRecentlyPolledAsync(
            activeIds.ToList(), ct).ConfigureAwait(false);
        var sortedIds = leastRecentlyPolled.AuthorityIds;

        // Emit the never-polled cohort size at cycle start so dashboards can detect
        // tc-ews7-style fairness regressions directly rather than via ad-hoc Cosmos
        // queries. Drains monotonically toward 0 as the Seed cycle rotates through
        // the cohort. See bd tc-ifdl.
        PollingMetrics.NeverPolledCount.Record(
            leastRecentlyPolled.NeverPolledCount,
            cycleTypeTag);

        // Emit the stalest LastPollTime seen across all active authorities so dashboards
        // can answer "how far behind is the pipeline?" directly. sortedIds is ordered
        // never-polled-first, then ascending LastPollTime, so sortedIds[0] is always
        // the stalest candidate. Emitting at cycle start captures the state the cycle
        // is about to work through — more useful as a backlog signal than post-cycle lag.
        if (sortedIds.Count > 0)
        {
            var oldestAuthorityId = sortedIds[0];
            var oldestState = await this.pollStateStore.GetAsync(oldestAuthorityId, ct).ConfigureAwait(false);
            var oldestLastPollTime = oldestState?.LastPollTime ?? DateTimeOffset.UnixEpoch;
            var neverPolled = oldestState is null;
            PollingMetrics.OldestHighWaterMarkAge.Record(
                (now - oldestLastPollTime).TotalSeconds,
                cycleTypeTag,
                new KeyValuePair<string, object?>("polling.authority_code", oldestAuthorityId),
                new KeyValuePair<string, object?>("never_polled", neverPolled ? "true" : "false"));
        }

        bool BudgetExhausted() => deadline.HasValue && this.timeProvider.GetUtcNow() >= deadline.Value;

        var count = 0;
        var authoritiesPolled = 0;
        var rateLimited = false;
        var authorityErrors = 0;
        var timeBounded = false;
        TimeSpan? rateLimitRetryAfter = null;
        foreach (var authorityId in sortedIds)
        {
            // Graceful timeout: when the worker's CTS (bounded to replicaTimeout - grace) fires,
            // break before starting a new authority so the partial result is returned cleanly.
            // See bd tc-qdtu — Container Apps replicaTimeout was SIGTERM-ing mid-loop and marking
            // otherwise-successful seed cycles as Failed. The soft HandlerBudget (docs/specs/
            // poll-handler-soft-budget.md) layers a second, shorter deadline so poll-sb runs
            // self-bound inside the 5-min Service Bus message lock.
            if (ct.IsCancellationRequested || BudgetExhausted())
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
            var existingHighWaterMark = now.AddDays(-1);
            var lastPageFetched = 0;
            int? firstPageTotal = null;

            var hadActiveCursor = false;
            try
            {
                var existingState = await this.pollStateStore.GetAsync(authorityId, ct).ConfigureAwait(false);
                existingHighWaterMark = existingState?.HighWaterMark ?? now.AddDays(-1);
                hadActiveCursor = existingState?.Cursor is not null;

                // Resume from a previously-saved cursor only when its recorded date matches
                // the HWM date we're about to query. If the HWM has advanced past the cursor's
                // date the cursor is stale and must be ignored (see tc-70kg / polling-resumable-cursor).
                // Overlap by one page (-1) to tolerate PlanIt page-shift between cycles.
                var startPage = 1;
                if (existingState?.Cursor is { } resumeCursor
                    && resumeCursor.DifferentStart == DateOnly.FromDateTime(existingHighWaterMark.UtcDateTime))
                {
                    startPage = Math.Max(1, resumeCursor.NextPage - 1);
                }

                var maxPages = this.options.MaxPagesPerAuthorityPerCycle;
                var pagesFetched = 0;
                var page = startPage;
                while (true)
                {
                    var pageResult = await this.planItClient.FetchApplicationsPageAsync(
                        authorityId, existingHighWaterMark, page, ct).ConfigureAwait(false);

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

                        // Decision-event dispatch: detect the transition from non-decision to
                        // a decision state (Permitted, Conditions, Rejected, Appealed) so we
                        // notify bookmark holders exactly once per application. First-time
                        // inserts that arrive already-decided count as a transition (existing
                        // is null). The check happens BEFORE upsert so we compare the persisted
                        // state, not the incoming one. Idempotency lives downstream in
                        // DispatchDecisionEventCommandHandler — one event per user per app —
                        // so re-dispatch on a same-decision-class change is harmless but we
                        // still gate on the transition to keep observability honest.
                        // See docs/specs/decision-state-vocabulary.md#dispatch.
                        var isNewDecision = IsDecisionState(application.AppState)
                            && !IsDecisionState(existing?.AppState);

                        await this.applicationRepository.UpsertAsync(application, ct).ConfigureAwait(false);

                        if (isNewDecision)
                        {
                            await this.decisionEventDispatcher.DispatchAsync(application, ct).ConfigureAwait(false);
                        }

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

                    if (ct.IsCancellationRequested || BudgetExhausted())
                    {
                        // Mid-pagination budget exhaustion — reuse the capHit cursor-save
                        // path so the next cycle resumes at lastPageFetched + 1, and flag
                        // the outer termination as TimeBounded. See docs/specs/
                        // poll-handler-soft-budget.md.
                        capHit = true;
                        timeBounded = true;
                        break;
                    }

                    page++;
                }

                completedSuccessfully = true;
            }
            catch (PlanItRateLimitException ex)
            {
                // 429 is an expected, handled outcome — skip the authority, increment
                // rate_limited, and (elsewhere below) save a resumable cursor so the
                // next cycle picks up where this one left off. Do NOT call AddException
                // or SetStatus(Error) here: that surfaces a routine throttle event as an
                // unhandled exception in App Insights (see bd tc-qc65).
                PollingMetrics.RateLimited.Add(1, cycleTypeTag);
                rateLimited = true;
                rateLimitRetryAfter = ex.RetryAfter;
                RecordRetryAfter(ex.RetryAfter, cycleTypeTag, authorityId);
                LogRateLimitStop(this.logger, authorityId, ex);
            }
            catch (HttpRequestException ex) when (ex.StatusCode == HttpStatusCode.TooManyRequests)
            {
                // Fallback for any 429 that arrives without our typed exception — treat
                // the same as PlanItRateLimitException but without a Retry-After hint.
                PollingMetrics.RateLimited.Add(1, cycleTypeTag);
                rateLimited = true;
                RecordRetryAfter(retryAfter: null, cycleTypeTag, authorityId);
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
                    // Cap-hit / mid-pagination 429: freeze the HighWaterMark at the
                    // previously-stored value so the next cycle resumes from the same
                    // different_start date. Advancing HWM mid-pagination would skip
                    // the tail of the result set. LastPollTime still advances to `now`
                    // so the scheduler rotates off this authority rather than re-selecting
                    // it next cycle — see docs/specs/poll-state-split-last-poll-time.md.
                    var nextPage = lastPageFetched + 1;
                    await this.pollStateStore.SaveAsync(
                        authorityId,
                        lastPollTime: now,
                        highWaterMark: existingHighWaterMark,
                        new PollCursor(
                            DateOnly.FromDateTime(existingHighWaterMark.UtcDateTime),
                            nextPage,
                            firstPageTotal),
                        ct).ConfigureAwait(false);
                    PollingMetrics.CursorAdvanced.Add(1, cycleTypeTag);
                    authorityActivity?.SetTag("polling.cursor.next_page", nextPage);
                }
                else
                {
                    // Natural end: advance HWM to max LastDifferent observed this cycle,
                    // fall back to the existing HWM when PlanIt returned zero apps (quiet
                    // authority). LastPollTime always stamps to `now` so quiet authorities
                    // drop to the back of the LRU queue immediately — see spec above.
                    var advancedHighWaterMark = highWaterMark ?? existingHighWaterMark;
                    await this.pollStateStore.SaveAsync(
                        authorityId,
                        lastPollTime: now,
                        highWaterMark: advancedHighWaterMark,
                        cursor: null,
                        ct).ConfigureAwait(false);

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
            authorityErrors,
            rateLimitRetryAfter);
    }

    /// <summary>
    /// Whether <paramref name="appState"/> is one of the four PlanIt strings that
    /// represent a final decision worth alerting on: Permitted (granted),
    /// Conditions (granted with conditions), Rejected (refused), Appealed
    /// (refusal under appeal). Withdrawn is terminal but does NOT trigger an
    /// alert. Comparison is case-insensitive to tolerate provider drift, and a
    /// null/empty state is treated as non-decision. See
    /// docs/specs/decision-state-vocabulary.md.
    /// </summary>
    private static bool IsDecisionState(string? appState)
    {
        if (string.IsNullOrEmpty(appState))
        {
            return false;
        }

        return string.Equals(appState, "Permitted", StringComparison.OrdinalIgnoreCase)
            || string.Equals(appState, "Conditions", StringComparison.OrdinalIgnoreCase)
            || string.Equals(appState, "Rejected", StringComparison.OrdinalIgnoreCase)
            || string.Equals(appState, "Appealed", StringComparison.OrdinalIgnoreCase);
    }

    private static void RecordRetryAfter(TimeSpan? retryAfter, KeyValuePair<string, object?> cycleTypeTag, int authorityId)
    {
        // Emit retry_after_seconds for every 429 — including 0 with header_present=false
        // when PlanIt omitted the header — so dashboards can distinguish "no header" from
        // "small backoff" and surface the actual Retry-After distribution. See bd tc-6nkn.
        var headerPresent = retryAfter.HasValue;
        var value = retryAfter?.TotalSeconds ?? 0;
        PollingMetrics.RetryAfterSeconds.Record(
            value,
            cycleTypeTag,
            new KeyValuePair<string, object?>("polling.authority_code", authorityId),
            new KeyValuePair<string, object?>("header_present", headerPresent ? "true" : "false"));
    }

#pragma warning disable SA1204
    [LoggerMessage(Level = LogLevel.Warning, Message = "Rate limited polling authority {AuthorityId}, stopping polling cycle")]
    private static partial void LogRateLimitStop(ILogger logger, int authorityId, Exception exception);

    [LoggerMessage(Level = LogLevel.Error, Message = "Error polling authority {AuthorityId}, skipping to next authority")]
    private static partial void LogAuthorityError(ILogger logger, int authorityId, Exception exception);
#pragma warning restore SA1204
}
