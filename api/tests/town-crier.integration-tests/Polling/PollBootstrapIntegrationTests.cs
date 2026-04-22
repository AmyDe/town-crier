using System.Diagnostics.CodeAnalysis;
using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.Polling;
using TownCrier.Application.Tests.Polling;
using TownCrier.Infrastructure.Polling;
using TownCrier.Infrastructure.ServiceBus;
using TownCrier.Infrastructure.Tests.Polling;

namespace TownCrier.IntegrationTests.Polling;

/// <summary>
/// End-to-end wiring for the bootstrap-only safety-net job (ADR 0024 amendment
/// 2026-04-22). Exercises the REAL <see cref="PollTriggerBootstrapper"/>, REAL
/// <see cref="ServiceBusPollTriggerQueue"/>, REAL
/// <see cref="ServiceBusPollTriggerQueueMetrics"/>, and REAL
/// <see cref="PollNextRunScheduler"/>, backed by a fake
/// <see cref="IServiceBusRestClient"/> at the transport boundary. The probe
/// is now a non-destructive management-API read of <c>countDetails</c>.
/// </summary>
[SuppressMessage(
    "Minor Code Smell",
    "S1075:URIs should not be hardcoded",
    Justification = "Test fixture URIs.")]
public sealed class PollBootstrapIntegrationTests
{
    private const string QueueName = "poll";
    private static readonly DateTimeOffset Now = new(2026, 4, 21, 12, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task Should_PublishBootstrapTrigger_When_QueueIsEmpty()
    {
        // Arrange — queue counts are both zero (management-API probe).
        var rest = new FakeServiceBusRestClient();
        rest.EnqueueDepth(active: 0, scheduled: 0);
        var options = new ServiceBusRestOptions { Namespace = "sb-test", QueueName = QueueName };
        var queue = new ServiceBusPollTriggerQueue(rest, options);
        var metrics = new ServiceBusPollTriggerQueueMetrics(rest, options);
        var scheduler = new PollNextRunScheduler(new PollNextRunSchedulerOptions(), new ZeroJitter());
        var bootstrapper = new PollTriggerBootstrapper(
            queue,
            metrics,
            scheduler,
            new FakeTimeProvider(Now),
            NullLogger<PollTriggerBootstrapper>.Instance);

        // Act
        var result = await bootstrapper.TryBootstrapAsync(CancellationToken.None);

        // Assert — a single publish at now + natural cadence.
        await Assert.That(result.Published).IsTrue();
        await Assert.That(result.ProbeFailed).IsFalse();
        await Assert.That(rest.PublishCalls).HasCount().EqualTo(1);
        await Assert.That(rest.PublishCalls[0].QueueName).IsEqualTo(QueueName);
        await Assert.That(rest.PublishCalls[0].ScheduledEnqueueTimeUtc)
            .IsEqualTo(Now + TimeSpan.FromMinutes(5));
    }

    [Test]
    public async Task Should_SkipPublish_When_QueueHasActiveMessage()
    {
        // Arrange — management-API probe returns active>0 (chain is alive).
        // Bootstrapper must NOT publish a duplicate.
        var rest = new FakeServiceBusRestClient();
        rest.EnqueueDepth(active: 1, scheduled: 0);

        var options = new ServiceBusRestOptions { Namespace = "sb-test", QueueName = QueueName };
        var queue = new ServiceBusPollTriggerQueue(rest, options);
        var metrics = new ServiceBusPollTriggerQueueMetrics(rest, options);
        var scheduler = new PollNextRunScheduler(new PollNextRunSchedulerOptions(), new ZeroJitter());
        var bootstrapper = new PollTriggerBootstrapper(
            queue,
            metrics,
            scheduler,
            new FakeTimeProvider(Now),
            NullLogger<PollTriggerBootstrapper>.Instance);

        // Act
        var result = await bootstrapper.TryBootstrapAsync(CancellationToken.None);

        // Assert — non-destructive probe, no publish.
        await Assert.That(result.Published).IsFalse();
        await Assert.That(result.ProbeFailed).IsFalse();
        await Assert.That(rest.PublishCalls).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_SkipPublish_When_QueueHasScheduledMessage()
    {
        // Arrange — scheduled (future-dated) message pending. The management-API
        // probe sees it (previously blind under PeekLock), so no double-publish.
        var rest = new FakeServiceBusRestClient();
        rest.EnqueueDepth(active: 0, scheduled: 1);

        var options = new ServiceBusRestOptions { Namespace = "sb-test", QueueName = QueueName };
        var queue = new ServiceBusPollTriggerQueue(rest, options);
        var metrics = new ServiceBusPollTriggerQueueMetrics(rest, options);
        var scheduler = new PollNextRunScheduler(new PollNextRunSchedulerOptions(), new ZeroJitter());
        var bootstrapper = new PollTriggerBootstrapper(
            queue,
            metrics,
            scheduler,
            new FakeTimeProvider(Now),
            NullLogger<PollTriggerBootstrapper>.Instance);

        // Act
        var result = await bootstrapper.TryBootstrapAsync(CancellationToken.None);

        // Assert — non-destructive probe, no publish.
        await Assert.That(result.Published).IsFalse();
        await Assert.That(result.ProbeFailed).IsFalse();
        await Assert.That(rest.PublishCalls).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_ReturnFailure_WithoutThrowing_When_TransportFails()
    {
        // Arrange — management-API probe uses a metrics fake that throws. The
        // safety-net's primary job (polling) must not be aborted.
        var rest = new FakeServiceBusRestClient();
        var options = new ServiceBusRestOptions { Namespace = "sb-test", QueueName = QueueName };
        var queue = new ServiceBusPollTriggerQueue(rest, options);
        var metrics = new FakePollTriggerQueueMetrics();
        metrics.EnqueueThrow(new HttpRequestException("management-api unreachable"));
        var scheduler = new PollNextRunScheduler(new PollNextRunSchedulerOptions(), new ZeroJitter());
        var bootstrapper = new PollTriggerBootstrapper(
            queue,
            metrics,
            scheduler,
            new FakeTimeProvider(Now),
            NullLogger<PollTriggerBootstrapper>.Instance);

        // Act — must not throw.
        var result = await bootstrapper.TryBootstrapAsync(CancellationToken.None);

        // Assert — failure absorbed, surfaced via result fields.
        await Assert.That(result.Published).IsFalse();
        await Assert.That(result.ProbeFailed).IsTrue();
        await Assert.That(rest.PublishCalls).HasCount().EqualTo(0);
    }

    private sealed class ZeroJitter : IPollJitter
    {
        public TimeSpan NextOffset(TimeSpan bound) => TimeSpan.Zero;
    }

    private sealed class FakeTimeProvider : TimeProvider
    {
        private readonly DateTimeOffset now;

        public FakeTimeProvider(DateTimeOffset now) => this.now = now;

        public override DateTimeOffset GetUtcNow() => this.now;
    }
}
