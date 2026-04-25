using System.Diagnostics.Metrics;
using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.Observability;
using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

[NotInParallel]
public sealed class PollPlanItCommandHandlerNeverPolledMetricTests
{
    [Test]
    public async Task Should_EmitNeverPolledCount_TaggedWithCycleTypeSeed_When5Of10AuthoritiesHaveNoState()
    {
        // Arrange — 10 active authorities, 5 have PollState, 5 do not.
        // The Seed cycle is the one that drains the never-polled cohort, so the
        // gauge must surface the size of that cohort tagged with cycle.type=seed.
        var now = new DateTimeOffset(2026, 4, 24, 13, 0, 0, TimeSpan.Zero);
        var timeProvider = new FakeTimeProvider(now);

        var authorityProvider = new FakeActiveAuthorityProvider();
        for (var id = 1; id <= 10; id++)
        {
            authorityProvider.Add(id);
        }

        var pollStateStore = new FakePollStateStore();
        for (var id = 1; id <= 5; id++)
        {
            pollStateStore.SetLastPollTime(id, now.AddHours(-1));
        }

        var planItClient = new FakePlanItClient();
        for (var id = 1; id <= 10; id++)
        {
            planItClient.Add(id, new PlanningApplicationBuilder().WithUid($"app-{id}").WithAreaId(id).Build());
        }

        var cycleSelector = new FakeCycleSelector(CycleType.Seed);
        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider,
            cycleSelector: cycleSelector,
            timeProvider: timeProvider);

        var recorded = CaptureNeverPolledMeasurements(out var listener);
        using (listener)
        {
            // Act
            await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);
        }

        // Assert
        await Assert.That(recorded).HasCount().EqualTo(1);
        await Assert.That(recorded[0].Value).IsEqualTo(5L);
        await Assert.That(recorded[0].CycleType).IsEqualTo("seed");
    }

    [Test]
    public async Task Should_EmitZero_When_AllAuthoritiesHaveBeenPolled()
    {
        // Arrange — every active authority already has a PollState document.
        // The drain end-state must report 0 so dashboards can confirm fairness
        // recovery rather than infer it from the absence of an emission.
        var now = new DateTimeOffset(2026, 4, 24, 13, 0, 0, TimeSpan.Zero);
        var timeProvider = new FakeTimeProvider(now);

        var authorityProvider = new FakeActiveAuthorityProvider();
        for (var id = 1; id <= 4; id++)
        {
            authorityProvider.Add(id);
        }

        var pollStateStore = new FakePollStateStore();
        for (var id = 1; id <= 4; id++)
        {
            pollStateStore.SetLastPollTime(id, now.AddHours(-1));
        }

        var planItClient = new FakePlanItClient();
        for (var id = 1; id <= 4; id++)
        {
            planItClient.Add(id, new PlanningApplicationBuilder().WithUid($"app-{id}").WithAreaId(id).Build());
        }

        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider,
            timeProvider: timeProvider);

        var recorded = CaptureNeverPolledMeasurements(out var listener);
        using (listener)
        {
            // Act
            await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);
        }

        // Assert
        await Assert.That(recorded).HasCount().EqualTo(1);
        await Assert.That(recorded[0].Value).IsEqualTo(0L);
    }

    [Test]
    public async Task Should_EmitTotalActiveCount_When_NoPollStateDocumentsExist()
    {
        // Arrange — a fresh deploy: no PollState documents anywhere. Every active
        // authority is in the never-polled cohort, so the gauge must equal the
        // active-authority count and provide an initial-state baseline.
        var now = new DateTimeOffset(2026, 4, 24, 13, 0, 0, TimeSpan.Zero);
        var timeProvider = new FakeTimeProvider(now);

        var authorityProvider = new FakeActiveAuthorityProvider();
        for (var id = 1; id <= 7; id++)
        {
            authorityProvider.Add(id);
        }

        var pollStateStore = new FakePollStateStore();

        var planItClient = new FakePlanItClient();
        for (var id = 1; id <= 7; id++)
        {
            planItClient.Add(id, new PlanningApplicationBuilder().WithUid($"app-{id}").WithAreaId(id).Build());
        }

        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider,
            timeProvider: timeProvider);

        var recorded = CaptureNeverPolledMeasurements(out var listener);
        using (listener)
        {
            // Act
            await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);
        }

        // Assert
        await Assert.That(recorded).HasCount().EqualTo(1);
        await Assert.That(recorded[0].Value).IsEqualTo(7L);
    }

    [Test]
    public async Task Should_TagCycleTypeWatched_When_WatchedCycle()
    {
        // Arrange — the Watched cycle reuses the same gauge; the cycle.type tag
        // must distinguish the two so dashboards can chart drain-by-cycle.
        var now = new DateTimeOffset(2026, 4, 24, 13, 0, 0, TimeSpan.Zero);
        var timeProvider = new FakeTimeProvider(now);

        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);

        var pollStateStore = new FakePollStateStore();
        pollStateStore.SetLastPollTime(100, now.AddHours(-1));

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-100").WithAreaId(100).Build());
        planItClient.Add(200, new PlanningApplicationBuilder().WithUid("app-200").WithAreaId(200).Build());

        var cycleSelector = new FakeCycleSelector(CycleType.Watched);
        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider,
            cycleSelector: cycleSelector,
            timeProvider: timeProvider);

        var recorded = CaptureNeverPolledMeasurements(out var listener);
        using (listener)
        {
            // Act
            await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);
        }

        // Assert
        await Assert.That(recorded).HasCount().EqualTo(1);
        await Assert.That(recorded[0].Value).IsEqualTo(1L);
        await Assert.That(recorded[0].CycleType).IsEqualTo("watched");
    }

    private static List<(long Value, string? CycleType)> CaptureNeverPolledMeasurements(out MeterListener listener)
    {
        var recorded = new List<(long Value, string? CycleType)>();
        listener = new MeterListener();
        listener.InstrumentPublished = (instrument, l) =>
        {
            if (instrument.Name == "towncrier.polling.never_polled_count")
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
        return recorded;
    }

    private static PollPlanItCommandHandler CreateHandler(
        FakePlanItClient? planItClient = null,
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
            planItClient ?? new FakePlanItClient(),
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
}
