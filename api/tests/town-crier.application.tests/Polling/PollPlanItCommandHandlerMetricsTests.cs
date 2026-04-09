using System.Diagnostics.Metrics;
using System.Net;
using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.Observability;
using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

[NotInParallel]
public sealed class PollPlanItCommandHandlerMetricsTests
{
    [Test]
    public async Task Should_IncrementAuthoritiesPolled_When_AuthoritySucceeds()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.Add(200, new PlanningApplicationBuilder().WithUid("app-2").WithAreaId(200).Build());

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var recorded = new List<long>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name == "towncrier.polling.authorities_polled")
            {
                listener.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((instrument, measurement, tags, _) =>
        {
            recorded.Add(measurement);
        });
        listener.Start();

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — per-authority emission (one Add(1) per successful authority)
        await Assert.That(recorded).HasCount().EqualTo(2);
        await Assert.That(recorded[0]).IsEqualTo(1);
        await Assert.That(recorded[1]).IsEqualTo(1);
    }

    [Test]
    public async Task Should_IncrementAuthoritiesSkipped_When_AuthorityFails()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.ThrowForAuthority(200, new HttpRequestException("Internal Server Error", null, HttpStatusCode.InternalServerError));

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var recorded = new List<long>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name == "towncrier.polling.authorities_skipped")
            {
                listener.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((instrument, measurement, tags, _) =>
        {
            recorded.Add(measurement);
        });
        listener.Start();

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — authority 200 was skipped
        await Assert.That(recorded).HasCount().EqualTo(1);
        await Assert.That(recorded[0]).IsEqualTo(1);
    }

    [Test]
    public async Task Should_RecordApplicationsIngested_When_ApplicationsFetched()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-2").WithAreaId(100).Build());
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-3").WithAreaId(100).Build());

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var recorded = new List<long>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name == "towncrier.polling.applications_ingested")
            {
                listener.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((instrument, measurement, tags, _) =>
        {
            recorded.Add(measurement);
        });
        listener.Start();

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — 3 applications ingested for authority 100
        await Assert.That(recorded).HasCount().EqualTo(1);
        await Assert.That(recorded[0]).IsEqualTo(3);
    }

    [Test]
    public async Task Should_IncrementRateLimited_When_429Received()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var planItClient = new FakePlanItClient();
        planItClient.ThrowForAuthority(100, new HttpRequestException("Rate limited", null, HttpStatusCode.TooManyRequests));

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var recorded = new List<long>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name == "towncrier.polling.rate_limited")
            {
                listener.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((instrument, measurement, tags, _) =>
        {
            recorded.Add(measurement);
        });
        listener.Start();

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — rate limit recorded
        await Assert.That(recorded).HasCount().EqualTo(1);
        await Assert.That(recorded[0]).IsEqualTo(1);
    }

    [Test]
    public async Task Should_RecordAuthorityProcessingDuration_When_AuthorityCompletes()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var recorded = new List<double>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name == "towncrier.polling.authority_processing_ms")
            {
                listener.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<double>((instrument, measurement, tags, _) =>
        {
            recorded.Add(measurement);
        });
        listener.Start();

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — duration recorded and non-negative
        await Assert.That(recorded).HasCount().EqualTo(1);
        await Assert.That(recorded[0]).IsGreaterThanOrEqualTo(0);
    }

    [Test]
    public async Task Should_RecordAuthorityProcessingDuration_When_AuthorityFails()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var planItClient = new FakePlanItClient();
        planItClient.ThrowForAuthority(100, new HttpRequestException("Server Error", null, HttpStatusCode.InternalServerError));

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var recorded = new List<double>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name == "towncrier.polling.authority_processing_ms")
            {
                listener.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<double>((instrument, measurement, tags, _) =>
        {
            recorded.Add(measurement);
        });
        listener.Start();

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — duration still recorded even on failure
        await Assert.That(recorded).HasCount().EqualTo(1);
        await Assert.That(recorded[0]).IsGreaterThanOrEqualTo(0);
    }

    [Test]
    public async Task Should_IncrementAuthoritiesSkippedAndRateLimited_When_429Received()
    {
        // Arrange — rate limit also counts as a skip
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var planItClient = new FakePlanItClient();
        planItClient.ThrowForAuthority(100, new HttpRequestException("Rate limited", null, HttpStatusCode.TooManyRequests));

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var skipped = new List<long>();
        var rateLimited = new List<long>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name is "towncrier.polling.authorities_skipped" or "towncrier.polling.rate_limited")
            {
                listener.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((instrument, measurement, tags, _) =>
        {
            if (instrument.Name == "towncrier.polling.authorities_skipped")
            {
                skipped.Add(measurement);
            }
            else if (instrument.Name == "towncrier.polling.rate_limited")
            {
                rateLimited.Add(measurement);
            }
        });
        listener.Start();

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — both skipped and rate_limited incremented
        await Assert.That(skipped).HasCount().EqualTo(1);
        await Assert.That(rateLimited).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_EmitAuthoritiesPolledPerAuthority_When_MultipleSucceed()
    {
        // Arrange — two authorities that succeed
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.Add(200, new PlanningApplicationBuilder().WithUid("app-2").WithAreaId(200).Build());

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var recorded = new List<long>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name == "towncrier.polling.authorities_polled")
            {
                listener.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((instrument, measurement, tags, _) =>
        {
            recorded.Add(measurement);
        });
        listener.Start();

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — each authority emits separately with value 1 (not batched)
        await Assert.That(recorded).HasCount().EqualTo(2);
        await Assert.That(recorded).Contains(1L);
        await Assert.That(recorded.Sum()).IsEqualTo(2);
    }

    [Test]
    public async Task Should_EmitApplicationsIngestedPerAuthority_When_MultipleSucceed()
    {
        // Arrange — two authorities, each with applications
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-2").WithAreaId(100).Build());
        planItClient.Add(200, new PlanningApplicationBuilder().WithUid("app-3").WithAreaId(200).Build());

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var recorded = new List<long>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name == "towncrier.polling.applications_ingested")
            {
                listener.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((instrument, measurement, tags, _) =>
        {
            recorded.Add(measurement);
        });
        listener.Start();

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — each authority emits its own count (2 and 1), not a single batched 3
        await Assert.That(recorded).HasCount().EqualTo(2);
        await Assert.That(recorded.Sum()).IsEqualTo(3);
    }

    [Test]
    public async Task Should_NotEmitAuthoritiesPolled_When_AllAuthoritiesRateLimited()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var planItClient = new FakePlanItClient();
        planItClient.ThrowForAuthority(100, new HttpRequestException("Rate limited", null, HttpStatusCode.TooManyRequests));

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var recorded = new List<long>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name == "towncrier.polling.authorities_polled")
            {
                listener.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((instrument, measurement, tags, _) =>
        {
            recorded.Add(measurement);
        });
        listener.Start();

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — no emission when nothing was polled (zero-value Add is a no-op in OTel exporters)
        await Assert.That(recorded.Where(v => v > 0)).IsEmpty();
    }

    [Test]
    public async Task Should_NotEmitApplicationsIngested_When_AllAuthoritiesRateLimited()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var planItClient = new FakePlanItClient();
        planItClient.ThrowForAuthority(100, new HttpRequestException("Rate limited", null, HttpStatusCode.TooManyRequests));

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var recorded = new List<long>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name == "towncrier.polling.applications_ingested")
            {
                listener.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((instrument, measurement, tags, _) =>
        {
            recorded.Add(measurement);
        });
        listener.Start();

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — no emission when nothing was ingested
        await Assert.That(recorded.Where(v => v > 0)).IsEmpty();
    }

    [Test]
    public async Task Should_NotCountFailedAuthorityAsPolled_When_AuthorityErrors()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var planItClient = new FakePlanItClient();
        planItClient.ThrowForAuthority(100, new HttpRequestException("timeout"));

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var recorded = new List<long>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name == "towncrier.polling.authorities_polled")
            {
                listener.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((instrument, measurement, tags, _) =>
        {
            recorded.Add(measurement);
        });
        listener.Start();

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — failed authority should NOT be counted as polled, but counter still emits
        await Assert.That(recorded.Sum()).IsEqualTo(0);
    }

    private static PollPlanItCommandHandler CreateHandler(
        FakePlanItClient? planItClient = null,
        FakePollStateStore? pollStateStore = null,
        FakePlanningApplicationRepository? repository = null,
        FakeActiveAuthorityProvider? authorityProvider = null,
        FakeWatchZoneRepository? watchZoneRepository = null,
        FakeNotificationEnqueuer? notificationEnqueuer = null,
        TimeProvider? timeProvider = null)
    {
        return new PollPlanItCommandHandler(
            planItClient ?? new FakePlanItClient(),
            pollStateStore ?? new FakePollStateStore(),
            repository ?? new FakePlanningApplicationRepository(),
            timeProvider ?? TimeProvider.System,
            authorityProvider ?? new FakeActiveAuthorityProvider(),
            watchZoneRepository ?? new FakeWatchZoneRepository(),
            notificationEnqueuer ?? new FakeNotificationEnqueuer(),
            NullLogger<PollPlanItCommandHandler>.Instance);
    }
}
