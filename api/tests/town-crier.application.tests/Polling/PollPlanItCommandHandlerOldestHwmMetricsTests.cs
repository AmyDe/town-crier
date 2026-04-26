using System.Diagnostics.Metrics;
using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.Observability;
using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

[NotInParallel]
public sealed class PollPlanItCommandHandlerOldestHwmMetricsTests
{
    [Test]
    public async Task Should_EmitOldestHwmAgeSeconds_Tagged_With_CycleTypeAndAuthorityCode()
    {
        // Arrange — two authorities, authority 100 has the oldest LastPollTime
        var now = new DateTimeOffset(2026, 4, 24, 13, 0, 0, TimeSpan.Zero);
        var timeProvider = new FakeTimeProvider(now);

        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);

        var pollStateStore = new FakePollStateStore();
        pollStateStore.SetLastPollTime(100, now.AddHours(-6)); // oldest
        pollStateStore.SetLastPollTime(200, now.AddHours(-1));

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.Add(200, new PlanningApplicationBuilder().WithUid("app-2").WithAreaId(200).Build());

        var cycleSelector = new FakeCycleSelector(CycleType.Watched);
        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider,
            cycleSelector: cycleSelector,
            timeProvider: timeProvider);

        var recorded = new List<(double Value, string? CycleType, string? AuthorityCode)>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name == "towncrier.polling.oldest_hwm_age_seconds")
            {
                listener.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<double>((instrument, measurement, tags, _) =>
        {
            string? cycleType = null;
            string? authorityCode = null;
            foreach (var tag in tags)
            {
                if (tag.Key == "cycle.type")
                {
                    cycleType = tag.Value?.ToString();
                }
                else if (tag.Key == "polling.authority_code")
                {
                    authorityCode = tag.Value?.ToString();
                }
            }

            recorded.Add((measurement, cycleType, authorityCode));
        });
        listener.Start();

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — one emission per cycle, authority 100 is the stalest (6h = 21600s)
        await Assert.That(recorded).HasCount().EqualTo(1);
        await Assert.That(recorded[0].Value).IsEqualTo(TimeSpan.FromHours(6).TotalSeconds);
        await Assert.That(recorded[0].CycleType).IsEqualTo("watched");
        await Assert.That(recorded[0].AuthorityCode).IsEqualTo("100");
    }

    [Test]
    public async Task Should_TagNeverPolledTrue_When_OldestAuthorityHasNoState()
    {
        // Arrange — authority 100 has never been polled (no state), authority 200 has state
        var now = new DateTimeOffset(2026, 4, 24, 13, 0, 0, TimeSpan.Zero);
        var timeProvider = new FakeTimeProvider(now);

        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);

        var pollStateStore = new FakePollStateStore();
        pollStateStore.SetLastPollTime(200, now.AddHours(-1));

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.Add(200, new PlanningApplicationBuilder().WithUid("app-2").WithAreaId(200).Build());

        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider,
            timeProvider: timeProvider);

        var recorded = new List<(double Value, string? NeverPolled, string? AuthorityCode)>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name == "towncrier.polling.oldest_hwm_age_seconds")
            {
                listener.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<double>((instrument, measurement, tags, _) =>
        {
            string? neverPolled = null;
            string? authorityCode = null;
            foreach (var tag in tags)
            {
                if (tag.Key == "never_polled")
                {
                    neverPolled = tag.Value?.ToString();
                }
                else if (tag.Key == "polling.authority_code")
                {
                    authorityCode = tag.Value?.ToString();
                }
            }

            recorded.Add((measurement, neverPolled, authorityCode));
        });
        listener.Start();

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — one emission, never_polled="true", age is (now - UnixEpoch)
        await Assert.That(recorded).HasCount().EqualTo(1);
        await Assert.That(recorded[0].NeverPolled).IsEqualTo("true");
        await Assert.That(recorded[0].AuthorityCode).IsEqualTo("100");
        await Assert.That(recorded[0].Value).IsEqualTo((now - DateTimeOffset.UnixEpoch).TotalSeconds);
    }

    [Test]
    public async Task Should_TagNeverPolledFalse_When_OldestAuthorityHasState()
    {
        // Arrange — authority 100 has LastPollTime one hour ago
        var now = new DateTimeOffset(2026, 4, 24, 13, 0, 0, TimeSpan.Zero);
        var timeProvider = new FakeTimeProvider(now);

        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var pollStateStore = new FakePollStateStore();
        pollStateStore.SetLastPollTime(100, now.AddHours(-1));

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());

        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider,
            timeProvider: timeProvider);

        var recordedTags = new List<string?>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name == "towncrier.polling.oldest_hwm_age_seconds")
            {
                listener.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<double>((_, _, tags, _) =>
        {
            foreach (var tag in tags)
            {
                if (tag.Key == "never_polled")
                {
                    recordedTags.Add(tag.Value?.ToString());
                }
            }
        });
        listener.Start();

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(recordedTags).HasCount().EqualTo(1);
        await Assert.That(recordedTags[0]).IsEqualTo("false");
    }

    [Test]
    public async Task Should_NotEmitOldestHwmAge_When_NoActiveAuthorities()
    {
        // Arrange — no active authorities
        var authorityProvider = new FakeActiveAuthorityProvider();

        var handler = CreateHandler(authorityProvider: authorityProvider);

        var recorded = new List<double>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name == "towncrier.polling.oldest_hwm_age_seconds")
            {
                listener.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<double>((_, measurement, _, _) =>
        {
            recorded.Add(measurement);
        });
        listener.Start();

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — nothing to emit when there are no authorities
        await Assert.That(recorded).IsEmpty();
    }

    private static PollPlanItCommandHandler CreateHandler(
        FakePlanItClient? planItClient = null,
        FakePollStateStore? pollStateStore = null,
        FakePlanningApplicationRepository? repository = null,
        FakeActiveAuthorityProvider? authorityProvider = null,
        FakeWatchZoneRepository? watchZoneRepository = null,
        FakeNotificationEnqueuer? notificationEnqueuer = null,
        FakeDecisionAlertDispatcher? decisionAlertDispatcher = null,
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
            decisionAlertDispatcher ?? new FakeDecisionAlertDispatcher(),
            cycleSelector ?? new FakeCycleSelector(CycleType.Watched),
            options ?? new PollingOptions(),
            NullLogger<PollPlanItCommandHandler>.Instance);
    }
}
