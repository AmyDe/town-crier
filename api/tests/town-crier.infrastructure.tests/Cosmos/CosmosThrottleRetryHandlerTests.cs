using System.Diagnostics.CodeAnalysis;
using System.Diagnostics.Metrics;
using System.Net;
using TownCrier.Infrastructure.Cosmos;
using TownCrier.Infrastructure.Observability;

namespace TownCrier.Infrastructure.Tests.Cosmos;

[SuppressMessage(
    "Reliability",
    "CA2000:Dispose objects before losing scope",
    Justification = "HttpClient lifetime managed by test")]
[SuppressMessage(
    "Major Code Smell",
    "S1075:URIs should not be hardcoded",
    Justification = "Stub base address for test only.")]
public sealed class CosmosThrottleRetryHandlerTests
{
    private static readonly Uri BaseAddress = new("https://test-cosmos.invalid");
    private static readonly Uri Path = new("/foo", UriKind.Relative);

    [Test]
    public async Task Should_RetryOn429_AndReturnSuccessfulResponse_When_RetrySucceeds()
    {
        var inner = new StubHttpHandler();
        inner.EnqueueResponse(
            HttpStatusCode.TooManyRequests,
            content: null,
            headers: [new("x-ms-retry-after-ms", "10")]);
        inner.EnqueueResponse(HttpStatusCode.OK, """{"ok":true}""");

        var delays = new List<TimeSpan>();
        var handler = new CosmosThrottleRetryHandler(
            maxAttempts: 3,
            totalWaitBudget: TimeSpan.FromMilliseconds(1500),
            perAttemptCap: TimeSpan.FromMilliseconds(750),
            jitter: _ => 0,
            delay: (d, _) =>
            {
                delays.Add(d);
                return Task.CompletedTask;
            })
        {
            InnerHandler = inner,
        };

        using var client = new HttpClient(handler) { BaseAddress = BaseAddress };

        using var response = await client.GetAsync(Path);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);
        await Assert.That(inner.SentRequests).HasCount().EqualTo(2);
        await Assert.That(delays).HasCount().EqualTo(1);
        await Assert.That(delays[0]).IsEqualTo(TimeSpan.FromMilliseconds(10));
    }

    [Test]
    public async Task Should_HonourXmsRetryAfterMsHeader_When_429()
    {
        var inner = new StubHttpHandler();
        inner.EnqueueResponse(
            HttpStatusCode.TooManyRequests,
            content: null,
            headers: [new("x-ms-retry-after-ms", "250")]);
        inner.EnqueueResponse(HttpStatusCode.OK);

        var delays = new List<TimeSpan>();
        var handler = new CosmosThrottleRetryHandler(
            maxAttempts: 3,
            totalWaitBudget: TimeSpan.FromMilliseconds(1500),
            perAttemptCap: TimeSpan.FromMilliseconds(750),
            jitter: _ => 0,
            delay: (d, _) =>
            {
                delays.Add(d);
                return Task.CompletedTask;
            })
        {
            InnerHandler = inner,
        };

        using var client = new HttpClient(handler) { BaseAddress = BaseAddress };
        using var response = await client.GetAsync(Path);

        await Assert.That(delays).HasCount().EqualTo(1);
        await Assert.That(delays[0]).IsEqualTo(TimeSpan.FromMilliseconds(250));
    }

    [Test]
    public async Task Should_CapPerAttemptDelay_When_HeaderExceedsCap()
    {
        var inner = new StubHttpHandler();
        inner.EnqueueResponse(
            HttpStatusCode.TooManyRequests,
            content: null,
            headers: [new("x-ms-retry-after-ms", "5000")]);
        inner.EnqueueResponse(HttpStatusCode.OK);

        var delays = new List<TimeSpan>();
        var handler = new CosmosThrottleRetryHandler(
            maxAttempts: 3,
            totalWaitBudget: TimeSpan.FromMilliseconds(1500),
            perAttemptCap: TimeSpan.FromMilliseconds(750),
            jitter: _ => 0,
            delay: (d, _) =>
            {
                delays.Add(d);
                return Task.CompletedTask;
            })
        {
            InnerHandler = inner,
        };

        using var client = new HttpClient(handler) { BaseAddress = BaseAddress };
        using var response = await client.GetAsync(Path);

        await Assert.That(delays).HasCount().EqualTo(1);
        await Assert.That(delays[0]).IsEqualTo(TimeSpan.FromMilliseconds(750));
    }

    [Test]
    public async Task Should_StopRetrying_When_MaxAttemptsExhausted()
    {
        var inner = new StubHttpHandler();
        for (var i = 0; i < 5; i++)
        {
            inner.EnqueueResponse(
                HttpStatusCode.TooManyRequests,
                content: null,
                headers: [new("x-ms-retry-after-ms", "10")]);
        }

        var handler = new CosmosThrottleRetryHandler(
            maxAttempts: 3,
            totalWaitBudget: TimeSpan.FromMilliseconds(1500),
            perAttemptCap: TimeSpan.FromMilliseconds(750),
            jitter: _ => 0,
            delay: (_, _) => Task.CompletedTask)
        {
            InnerHandler = inner,
        };

        using var client = new HttpClient(handler) { BaseAddress = BaseAddress };
        using var response = await client.GetAsync(Path);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.TooManyRequests);
        await Assert.That(inner.SentRequests).HasCount().EqualTo(3);
    }

    [Test]
    public async Task Should_StopRetrying_When_TotalWaitBudgetExceeded()
    {
        var inner = new StubHttpHandler();
        for (var i = 0; i < 5; i++)
        {
            inner.EnqueueResponse(
                HttpStatusCode.TooManyRequests,
                content: null,
                headers: [new("x-ms-retry-after-ms", "700")]);
        }

        var delays = new List<TimeSpan>();
        var handler = new CosmosThrottleRetryHandler(
            maxAttempts: 5,
            totalWaitBudget: TimeSpan.FromMilliseconds(1500),
            perAttemptCap: TimeSpan.FromMilliseconds(750),
            jitter: _ => 0,
            delay: (d, _) =>
            {
                delays.Add(d);
                return Task.CompletedTask;
            })
        {
            InnerHandler = inner,
        };

        using var client = new HttpClient(handler) { BaseAddress = BaseAddress };
        using var response = await client.GetAsync(Path);

        // After two retries we have spent 1400ms; a third would bust the 1500ms budget.
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.TooManyRequests);
        await Assert.That(delays).HasCount().EqualTo(2);
        await Assert.That(inner.SentRequests).HasCount().EqualTo(3);
    }

    [Test]
    public async Task Should_NotRetry_When_StatusIsNot429()
    {
        var inner = new StubHttpHandler();
        inner.EnqueueResponse(HttpStatusCode.InternalServerError);

        var handler = new CosmosThrottleRetryHandler(
            maxAttempts: 3,
            totalWaitBudget: TimeSpan.FromMilliseconds(1500),
            perAttemptCap: TimeSpan.FromMilliseconds(750),
            jitter: _ => 0,
            delay: (_, _) => Task.CompletedTask)
        {
            InnerHandler = inner,
        };

        using var client = new HttpClient(handler) { BaseAddress = BaseAddress };
        using var response = await client.GetAsync(Path);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.InternalServerError);
        await Assert.That(inner.SentRequests).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_FallBackToDefaultDelay_When_429HasNoRetryAfterHeader()
    {
        var inner = new StubHttpHandler();
        inner.EnqueueResponse(HttpStatusCode.TooManyRequests);
        inner.EnqueueResponse(HttpStatusCode.OK);

        var delays = new List<TimeSpan>();
        var handler = new CosmosThrottleRetryHandler(
            maxAttempts: 3,
            totalWaitBudget: TimeSpan.FromMilliseconds(1500),
            perAttemptCap: TimeSpan.FromMilliseconds(750),
            jitter: _ => 0,
            delay: (d, _) =>
            {
                delays.Add(d);
                return Task.CompletedTask;
            })
        {
            InnerHandler = inner,
        };

        using var client = new HttpClient(handler) { BaseAddress = BaseAddress };
        using var response = await client.GetAsync(Path);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);
        await Assert.That(delays).HasCount().EqualTo(1);

        // Default fallback should be > 0 and within the per-attempt cap.
        await Assert.That(delays[0]).IsGreaterThan(TimeSpan.Zero);
        await Assert.That(delays[0]).IsLessThanOrEqualTo(TimeSpan.FromMilliseconds(750));
    }

    [Test]
    [NotInParallel]
    public async Task Should_IncrementThrottleCounter_When_429ResponseObserved()
    {
        // Arrange — one 429 followed by a 200 so we observe a single throttle
        // event regardless of whether the retry succeeds.
        var inner = new StubHttpHandler();
        inner.EnqueueResponse(
            HttpStatusCode.TooManyRequests,
            content: null,
            headers: [new("x-ms-retry-after-ms", "10")]);
        inner.EnqueueResponse(HttpStatusCode.OK, """{"ok":true}""");

        var throttleMeasurements = new List<long>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, l) =>
        {
            if (instrument.Meter.Name == CosmosInstrumentation.MeterName
                && instrument.Name == "towncrier.cosmos.throttles")
            {
                l.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>(
            (_, measurement, _, _) => throttleMeasurements.Add(measurement));
        listener.Start();

        var handler = new CosmosThrottleRetryHandler(
            maxAttempts: 3,
            totalWaitBudget: TimeSpan.FromMilliseconds(1500),
            perAttemptCap: TimeSpan.FromMilliseconds(750),
            jitter: _ => 0,
            delay: (_, _) => Task.CompletedTask)
        {
            InnerHandler = inner,
        };

        using var client = new HttpClient(handler) { BaseAddress = BaseAddress };

        // Act
        using var response = await client.GetAsync(Path);

        // Assert
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);
        await Assert.That(throttleMeasurements).HasCount().EqualTo(1);
        await Assert.That(throttleMeasurements[0]).IsEqualTo(1L);
    }

    [Test]
    [NotInParallel]
    public async Task Should_IncrementThrottleCounterPerObserved429_When_MultipleRetriesFire()
    {
        // Arrange — two 429s (both retries fire), then a 200.
        var inner = new StubHttpHandler();
        inner.EnqueueResponse(
            HttpStatusCode.TooManyRequests,
            content: null,
            headers: [new("x-ms-retry-after-ms", "10")]);
        inner.EnqueueResponse(
            HttpStatusCode.TooManyRequests,
            content: null,
            headers: [new("x-ms-retry-after-ms", "10")]);
        inner.EnqueueResponse(HttpStatusCode.OK);

        var throttleMeasurements = new List<long>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, l) =>
        {
            if (instrument.Meter.Name == CosmosInstrumentation.MeterName
                && instrument.Name == "towncrier.cosmos.throttles")
            {
                l.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>(
            (_, measurement, _, _) => throttleMeasurements.Add(measurement));
        listener.Start();

        var handler = new CosmosThrottleRetryHandler(
            maxAttempts: 5,
            totalWaitBudget: TimeSpan.FromMilliseconds(1500),
            perAttemptCap: TimeSpan.FromMilliseconds(750),
            jitter: _ => 0,
            delay: (_, _) => Task.CompletedTask)
        {
            InnerHandler = inner,
        };

        using var client = new HttpClient(handler) { BaseAddress = BaseAddress };

        // Act
        using var response = await client.GetAsync(Path);

        // Assert — every 429 observed by the handler increments the counter,
        // including ones that are subsequently retried successfully.
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);
        await Assert.That(throttleMeasurements).HasCount().EqualTo(2);
    }

    [Test]
    [NotInParallel]
    public async Task Should_NotIncrementThrottleCounter_When_NoThrottleObserved()
    {
        // Arrange — clean 200, the counter must not fire.
        var inner = new StubHttpHandler();
        inner.EnqueueResponse(HttpStatusCode.OK);

        var throttleMeasurements = new List<long>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, l) =>
        {
            if (instrument.Meter.Name == CosmosInstrumentation.MeterName
                && instrument.Name == "towncrier.cosmos.throttles")
            {
                l.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>(
            (_, measurement, _, _) => throttleMeasurements.Add(measurement));
        listener.Start();

        var handler = new CosmosThrottleRetryHandler(
            maxAttempts: 3,
            totalWaitBudget: TimeSpan.FromMilliseconds(1500),
            perAttemptCap: TimeSpan.FromMilliseconds(750),
            jitter: _ => 0,
            delay: (_, _) => Task.CompletedTask)
        {
            InnerHandler = inner,
        };

        using var client = new HttpClient(handler) { BaseAddress = BaseAddress };

        // Act
        using var response = await client.GetAsync(Path);

        // Assert
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);
        await Assert.That(throttleMeasurements).IsEmpty();
    }

    [Test]
    public async Task Should_AddJitter_When_RetryingOn429()
    {
        var inner = new StubHttpHandler();
        inner.EnqueueResponse(
            HttpStatusCode.TooManyRequests,
            content: null,
            headers: [new("x-ms-retry-after-ms", "100")]);
        inner.EnqueueResponse(HttpStatusCode.OK);

        var delays = new List<TimeSpan>();
        var handler = new CosmosThrottleRetryHandler(
            maxAttempts: 3,
            totalWaitBudget: TimeSpan.FromMilliseconds(1500),
            perAttemptCap: TimeSpan.FromMilliseconds(750),
            jitter: _ => 25,
            delay: (d, _) =>
            {
                delays.Add(d);
                return Task.CompletedTask;
            })
        {
            InnerHandler = inner,
        };

        using var client = new HttpClient(handler) { BaseAddress = BaseAddress };
        using var response = await client.GetAsync(Path);

        await Assert.That(delays).HasCount().EqualTo(1);
        await Assert.That(delays[0]).IsEqualTo(TimeSpan.FromMilliseconds(125));
    }
}
