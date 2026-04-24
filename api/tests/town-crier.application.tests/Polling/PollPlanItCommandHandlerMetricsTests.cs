using System.Diagnostics.Metrics;
using System.Net;
using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.Observability;
using TownCrier.Application.PlanIt;
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

    [Test]
    public async Task Should_TagAuthoritiesPolled_With_CycleType()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        var cycleSelector = new FakeCycleSelector(CycleType.Seed);
        var handler = CreateHandler(
            planItClient: planItClient,
            authorityProvider: authorityProvider,
            cycleSelector: cycleSelector);

        var recordedTags = new List<string?>();
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
            foreach (var tag in tags)
            {
                if (tag.Key == "cycle.type")
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
        await Assert.That(recordedTags[0]).IsEqualTo("seed");
        await Assert.That(cycleSelector.GetCurrentCallCount).IsEqualTo(1);
    }

    [Test]
    public async Task Should_EmitCyclesCompleted_With_NaturalTermination()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());

        var cycleSelector = new FakeCycleSelector(CycleType.Seed);
        var handler = CreateHandler(
            planItClient: planItClient,
            authorityProvider: authorityProvider,
            cycleSelector: cycleSelector);

        var recorded = new List<(long Value, string? CycleType, string? Termination)>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name == "towncrier.polling.cycles_completed")
            {
                listener.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((instrument, measurement, tags, _) =>
        {
            string? cycleType = null;
            string? termination = null;
            foreach (var tag in tags)
            {
                if (tag.Key == "cycle.type")
                {
                    cycleType = tag.Value?.ToString();
                }
                else if (tag.Key == "termination")
                {
                    termination = tag.Value?.ToString();
                }
            }

            recorded.Add((measurement, cycleType, termination));
        });
        listener.Start();

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(recorded).HasCount().EqualTo(1);
        await Assert.That(recorded[0].Value).IsEqualTo(1);
        await Assert.That(recorded[0].CycleType).IsEqualTo("seed");
        await Assert.That(recorded[0].Termination).IsEqualTo("natural");
    }

    [Test]
    public async Task Should_EmitCyclesCompleted_With_TimeBoundedTermination()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.Add(200, new PlanningApplicationBuilder().WithUid("app-2").WithAreaId(200).Build());

        using var cts = new CancellationTokenSource();
        var pollStateStore = new FakePollStateStore { OnSave = (_, _) => cts.Cancel() };

        var cycleSelector = new FakeCycleSelector(CycleType.Seed);
        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider,
            cycleSelector: cycleSelector);

        var terminations = new List<string?>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name == "towncrier.polling.cycles_completed")
            {
                listener.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((_, _, tags, _) =>
        {
            foreach (var tag in tags)
            {
                if (tag.Key == "termination")
                {
                    terminations.Add(tag.Value?.ToString());
                }
            }
        });
        listener.Start();

        await handler.HandleAsync(new PollPlanItCommand(), cts.Token);

        await Assert.That(terminations).HasCount().EqualTo(1);
        await Assert.That(terminations[0]).IsEqualTo("time_bounded");
    }

    [Test]
    public async Task Should_EmitCyclesCompleted_With_RateLimitedTermination()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var planItClient = new FakePlanItClient();
        planItClient.ThrowForAuthority(100, new HttpRequestException("Rate limited", null, HttpStatusCode.TooManyRequests));

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var terminations = new List<string?>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name == "towncrier.polling.cycles_completed")
            {
                listener.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((_, _, tags, _) =>
        {
            foreach (var tag in tags)
            {
                if (tag.Key == "termination")
                {
                    terminations.Add(tag.Value?.ToString());
                }
            }
        });
        listener.Start();

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(terminations).HasCount().EqualTo(1);
        await Assert.That(terminations[0]).IsEqualTo("rate_limited");
    }

    [Test]
    public async Task Should_RecordRetryAfter_When_RateLimitedWithHeaderPresent()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var planItClient = new FakePlanItClient();
        var retryAfter = TimeSpan.FromSeconds(75);
        planItClient.ThrowForAuthority(100, new PlanItRateLimitException(retryAfter));

        var cycleSelector = new FakeCycleSelector(CycleType.Seed);
        var handler = CreateHandler(
            planItClient: planItClient,
            authorityProvider: authorityProvider,
            cycleSelector: cycleSelector);

        var recorded = new List<(double Value, string? CycleType, string? HeaderPresent, string? AuthorityCode)>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name == "towncrier.polling.retry_after_seconds")
            {
                listener.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<double>((_, measurement, tags, _) =>
        {
            string? cycleType = null;
            string? headerPresent = null;
            string? authorityCode = null;
            foreach (var tag in tags)
            {
                if (tag.Key == "cycle.type")
                {
                    cycleType = tag.Value?.ToString();
                }
                else if (tag.Key == "header_present")
                {
                    headerPresent = tag.Value?.ToString();
                }
                else if (tag.Key == "polling.authority_code")
                {
                    authorityCode = tag.Value?.ToString();
                }
            }

            recorded.Add((measurement, cycleType, headerPresent, authorityCode));
        });
        listener.Start();

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — retry_after value emitted with header_present=true and authority tag
        await Assert.That(recorded).HasCount().EqualTo(1);
        await Assert.That(recorded[0].Value).IsEqualTo(75);
        await Assert.That(recorded[0].CycleType).IsEqualTo("seed");
        await Assert.That(recorded[0].HeaderPresent).IsEqualTo("true");
        await Assert.That(recorded[0].AuthorityCode).IsEqualTo("100");
    }

    [Test]
    public async Task Should_RecordRetryAfterZero_When_RateLimitedWithHeaderAbsent()
    {
        // Arrange — PlanItRateLimitException with no Retry-After parsed
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(200);

        var planItClient = new FakePlanItClient();
        planItClient.ThrowForAuthority(200, new PlanItRateLimitException(retryAfter: null));

        var cycleSelector = new FakeCycleSelector(CycleType.Watched);
        var handler = CreateHandler(
            planItClient: planItClient,
            authorityProvider: authorityProvider,
            cycleSelector: cycleSelector);

        var recorded = new List<(double Value, string? CycleType, string? HeaderPresent, string? AuthorityCode)>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name == "towncrier.polling.retry_after_seconds")
            {
                listener.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<double>((_, measurement, tags, _) =>
        {
            string? cycleType = null;
            string? headerPresent = null;
            string? authorityCode = null;
            foreach (var tag in tags)
            {
                if (tag.Key == "cycle.type")
                {
                    cycleType = tag.Value?.ToString();
                }
                else if (tag.Key == "header_present")
                {
                    headerPresent = tag.Value?.ToString();
                }
                else if (tag.Key == "polling.authority_code")
                {
                    authorityCode = tag.Value?.ToString();
                }
            }

            recorded.Add((measurement, cycleType, headerPresent, authorityCode));
        });
        listener.Start();

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — emit a 0-value sample with header_present=false so the metric is
        // present in dashboards even when PlanIt omits the Retry-After header.
        await Assert.That(recorded).HasCount().EqualTo(1);
        await Assert.That(recorded[0].Value).IsEqualTo(0);
        await Assert.That(recorded[0].CycleType).IsEqualTo("watched");
        await Assert.That(recorded[0].HeaderPresent).IsEqualTo("false");
        await Assert.That(recorded[0].AuthorityCode).IsEqualTo("200");
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
