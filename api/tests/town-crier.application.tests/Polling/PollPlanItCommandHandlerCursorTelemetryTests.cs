using System.Diagnostics;
using System.Diagnostics.Metrics;
using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.Observability;
using TownCrier.Application.PlanIt;
using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

[NotInParallel]
public sealed class PollPlanItCommandHandlerCursorTelemetryTests
{
    [Test]
    public async Task Should_EmitAuthorityTotal_When_FirstPageReturnsTotal()
    {
        // Arrange — authority 100 has 7200 matching apps per PlanIt total.
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.TotalOverride = 7200;

        var handler = CreateHandler(
            planItClient: planItClient,
            authorityProvider: authorityProvider,
            cycleSelector: new FakeCycleSelector(CycleType.Seed));

        var recorded = new List<(long Value, string? CycleType, int? AuthorityCode)>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, l) =>
        {
            if (instrument.Name == "towncrier.polling.authority_total")
            {
                l.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((_, measurement, tags, _) =>
        {
            string? cycleType = null;
            int? authorityCode = null;
            foreach (var tag in tags)
            {
                if (tag.Key == "cycle.type")
                {
                    cycleType = tag.Value?.ToString();
                }
                else if (tag.Key == "polling.authority_code")
                {
                    authorityCode = tag.Value as int?;
                }
            }

            recorded.Add((measurement, cycleType, authorityCode));
        });
        listener.Start();

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(recorded).HasCount().EqualTo(1);
        await Assert.That(recorded[0].Value).IsEqualTo(7200);
        await Assert.That(recorded[0].CycleType).IsEqualTo("seed");
        await Assert.That(recorded[0].AuthorityCode).IsEqualTo(100);
    }

    [Test]
    public async Task Should_NotEmitAuthorityTotal_When_PageReturnsNullTotal()
    {
        // Arrange — FakePlanItClient derives Total from seeded apps count (non-null).
        // Tests an authority with zero apps and no override → Total is 0 not null.
        // Use a dedicated fake-subclass pattern: seed nothing, set TotalOverride=null,
        // and use the fake's default behaviour (total=allApps.Count=0). authority_total
        // should still emit (0 is a valid total) but we test the "Total==null → no emit"
        // path by using a planItClient that returns null.
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var planItClient = new NullTotalPlanItClient();
        var handler = CreateHandler(
            planItClient: planItClient,
            authorityProvider: authorityProvider);

        var recorded = new List<long>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, l) =>
        {
            if (instrument.Name == "towncrier.polling.authority_total")
            {
                l.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((_, measurement, _, _) => recorded.Add(measurement));
        listener.Start();

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — null Total means we don't emit authority_total.
        await Assert.That(recorded).IsEmpty();
    }

    [Test]
    public async Task Should_IncrementCursorAdvanced_When_NonNullCursorSaved()
    {
        // Arrange — 5 apps, page size 100, cap=1. First page returns 5 apps but
        // cap hits → cursor saved. Cursor "advanced" counter increments.
        // We use ThrowAfterYielding to force the page-cap path.
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var planItClient = new FakePlanItClient();

        // Seed a full page so HasMorePages=true, then cap=1 bails after page 1.
        for (var i = 0; i < FakePlanItClient.PageSize; i++)
        {
            planItClient.Add(100, new PlanningApplicationBuilder()
                .WithUid($"app-{i}")
                .WithAreaId(100)
                .Build());
        }

        var handler = CreateHandler(
            planItClient: planItClient,
            authorityProvider: authorityProvider,
            options: new PollingOptions { MaxPagesPerAuthorityPerCycle = 1 },
            cycleSelector: new FakeCycleSelector(CycleType.Seed));

        var recorded = new List<(long Value, string? CycleType)>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, l) =>
        {
            if (instrument.Name == "towncrier.polling.cursor_advanced")
            {
                l.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((_, measurement, tags, _) =>
        {
            string? cycleType = null;
            foreach (var tag in tags)
            {
                if (tag.Key == "cycle.type")
                {
                    cycleType = tag.Value?.ToString();
                }
            }

            recorded.Add((measurement, cycleType));
        });
        listener.Start();

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(recorded).HasCount().EqualTo(1);
        await Assert.That(recorded[0].Value).IsEqualTo(1);
        await Assert.That(recorded[0].CycleType).IsEqualTo("seed");
    }

    [Test]
    public async Task Should_IncrementCursorCleared_When_ActiveCursorClearedAtNaturalEnd()
    {
        // Arrange — authority has an active cursor from a prior cycle; this cycle
        // reaches natural end (only 1 app, HasMorePages=false). Cursor must be
        // cleared and the counter incremented exactly once.
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var lastPollTime = DateTimeOffset.UtcNow.AddDays(-1);
        var highWaterMark = lastPollTime;
        var pollStateStore = new FakePollStateStore();
        pollStateStore.SetState(100, new PollState(
            lastPollTime,
            highWaterMark,
            new PollCursor(DateOnly.FromDateTime(highWaterMark.UtcDateTime), NextPage: 4, KnownTotal: 7200)));

        var planItClient = new FakePlanItClient();

        // 1 app returned → HasMorePages=false → natural end.
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());

        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider,
            cycleSelector: new FakeCycleSelector(CycleType.Seed));

        var recorded = new List<(long Value, string? CycleType)>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, l) =>
        {
            if (instrument.Name == "towncrier.polling.cursor_cleared")
            {
                l.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((_, measurement, tags, _) =>
        {
            string? cycleType = null;
            foreach (var tag in tags)
            {
                if (tag.Key == "cycle.type")
                {
                    cycleType = tag.Value?.ToString();
                }
            }

            recorded.Add((measurement, cycleType));
        });
        listener.Start();

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(recorded).HasCount().EqualTo(1);
        await Assert.That(recorded[0].Value).IsEqualTo(1);
        await Assert.That(recorded[0].CycleType).IsEqualTo("seed");
    }

    [Test]
    public async Task Should_NotIncrementCursorCleared_When_NoPriorCursorAndNaturalEnd()
    {
        // Arrange — no prior cursor, natural end. Must NOT emit cursor_cleared
        // (nothing was cleared).
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());

        var handler = CreateHandler(
            planItClient: planItClient,
            authorityProvider: authorityProvider);

        var recorded = new List<long>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, l) =>
        {
            if (instrument.Name == "towncrier.polling.cursor_cleared")
            {
                l.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((_, measurement, _, _) => recorded.Add(measurement));
        listener.Start();

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(recorded).IsEmpty();
    }

    [Test]
    public async Task Should_SetCursorNextPageActivityTag_When_CursorSavedOnCapHit()
    {
        // Arrange — force cap hit so cursor is saved at NextPage=2.
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var planItClient = new FakePlanItClient();
        for (var i = 0; i < FakePlanItClient.PageSize; i++)
        {
            planItClient.Add(100, new PlanningApplicationBuilder()
                .WithUid($"app-{i}")
                .WithAreaId(100)
                .Build());
        }

        var handler = CreateHandler(
            planItClient: planItClient,
            authorityProvider: authorityProvider,
            options: new PollingOptions { MaxPagesPerAuthorityPerCycle = 1 });

        var stoppedActivities = new List<Activity>();
        using var listener = new ActivityListener
        {
            ShouldListenTo = source => source.Name == PollingInstrumentation.ActivitySourceName,
            Sample = (ref ActivityCreationOptions<ActivityContext> _) => ActivitySamplingResult.AllDataAndRecorded,
            ActivityStopped = activity => stoppedActivities.Add(activity),
        };
        ActivitySource.AddActivityListener(listener);

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        var authorityActivity = stoppedActivities.Find(a => a.DisplayName == "Poll Authority");
        await Assert.That(authorityActivity).IsNotNull();
        var nextPageTag = authorityActivity!.TagObjects.FirstOrDefault(t => t.Key == "polling.cursor.next_page");
        await Assert.That(nextPageTag.Value).IsEqualTo(2);
    }

    [Test]
    public async Task Should_SetAuthorityTotalActivityTag_When_FirstPageReturnsTotal()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.TotalOverride = 7200;

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var stoppedActivities = new List<Activity>();
        using var listener = new ActivityListener
        {
            ShouldListenTo = source => source.Name == PollingInstrumentation.ActivitySourceName,
            Sample = (ref ActivityCreationOptions<ActivityContext> _) => ActivitySamplingResult.AllDataAndRecorded,
            ActivityStopped = activity => stoppedActivities.Add(activity),
        };
        ActivitySource.AddActivityListener(listener);

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        var authorityActivity = stoppedActivities.Find(a => a.DisplayName == "Poll Authority");
        await Assert.That(authorityActivity).IsNotNull();
        var totalTag = authorityActivity!.TagObjects.FirstOrDefault(t => t.Key == "polling.authority_total");
        await Assert.That(totalTag.Value).IsEqualTo(7200);
    }

    private static PollPlanItCommandHandler CreateHandler(
        FakePlanItClient? planItClient = null,
        IPlanItClient? planItClientOverride = null,
        FakePollStateStore? pollStateStore = null,
        FakePlanningApplicationRepository? repository = null,
        FakeActiveAuthorityProvider? authorityProvider = null,
        FakeWatchZoneRepository? watchZoneRepository = null,
        FakeNotificationEnqueuer? notificationEnqueuer = null,
        TimeProvider? timeProvider = null,
        ICycleSelector? cycleSelector = null,
        PollingOptions? options = null)
    {
        return new PollPlanItCommandHandler(
            planItClientOverride ?? planItClient ?? new FakePlanItClient(),
            pollStateStore ?? new FakePollStateStore(),
            repository ?? new FakePlanningApplicationRepository(),
            timeProvider ?? TimeProvider.System,
            authorityProvider ?? new FakeActiveAuthorityProvider(),
            watchZoneRepository ?? new FakeWatchZoneRepository(),
            notificationEnqueuer ?? new FakeNotificationEnqueuer(),
            cycleSelector ?? new FakeCycleSelector(CycleType.Watched),
            options ?? new PollingOptions(),
            NullLogger<PollPlanItCommandHandler>.Instance);
    }

    // Overload so IPlanItClient substitutes are accepted in tests that need
    // behaviour not expressible through FakePlanItClient.
    private static PollPlanItCommandHandler CreateHandler(
        IPlanItClient planItClient,
        FakeActiveAuthorityProvider authorityProvider,
        FakePollStateStore? pollStateStore = null,
        FakePlanningApplicationRepository? repository = null,
        FakeWatchZoneRepository? watchZoneRepository = null,
        FakeNotificationEnqueuer? notificationEnqueuer = null,
        TimeProvider? timeProvider = null,
        ICycleSelector? cycleSelector = null,
        PollingOptions? options = null)
    {
        return new PollPlanItCommandHandler(
            planItClient,
            pollStateStore ?? new FakePollStateStore(),
            repository ?? new FakePlanningApplicationRepository(),
            timeProvider ?? TimeProvider.System,
            authorityProvider,
            watchZoneRepository ?? new FakeWatchZoneRepository(),
            notificationEnqueuer ?? new FakeNotificationEnqueuer(),
            cycleSelector ?? new FakeCycleSelector(CycleType.Watched),
            options ?? new PollingOptions(),
            NullLogger<PollPlanItCommandHandler>.Instance);
    }

    /// <summary>
    /// Stub client that returns pages with a <c>null</c> <see cref="FetchPageResult.Total"/>.
    /// Used to exercise the "don't emit authority_total when Total is null" path.
    /// </summary>
    private sealed class NullTotalPlanItClient : IPlanItClient
    {
        public Task<FetchPageResult> FetchApplicationsPageAsync(
            int authorityId,
            DateTimeOffset? differentStart,
            int page,
            CancellationToken ct)
        {
            return Task.FromResult(new FetchPageResult(
                page,
                Applications: [],
                Total: null,
                HasMorePages: false));
        }

        public Task<PlanItSearchResult> SearchApplicationsAsync(
            string searchText,
            int authorityId,
            int page,
            CancellationToken ct)
        {
            return Task.FromResult(new PlanItSearchResult([], 0));
        }
    }
}
