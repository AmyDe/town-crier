using System.Diagnostics.CodeAnalysis;
using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.Polling;
using TownCrier.Application.Tests.Polling;
using TownCrier.Infrastructure.Polling;
using TownCrier.Infrastructure.ServiceBus;
using TownCrier.Infrastructure.Tests.Polling;

namespace TownCrier.IntegrationTests.Polling;

/// <summary>
/// End-to-end wiring for the safety-net reseed (bd tc-tdgf). Exercises the REAL
/// <see cref="PollTriggerBootstrapper"/>, REAL <see cref="ServiceBusPollTriggerQueue"/>,
/// and REAL <see cref="PollNextRunScheduler"/> backed by a fake
/// <see cref="IServiceBusRestClient"/> at the transport boundary.
/// </summary>
[SuppressMessage(
    "Minor Code Smell",
    "S1075:URIs should not be hardcoded",
    Justification = "Test fixture URIs.")]
public sealed class SafetyNetBootstrapIntegrationTests
{
    private const string QueueName = "poll";
    private static readonly DateTimeOffset Now = new(2026, 4, 21, 12, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task Should_PublishBootstrapTrigger_When_QueueIsEmpty()
    {
        // Arrange — queue is empty, REST client returns null on receive.
        var rest = new FakeServiceBusRestClient();
        var options = new ServiceBusRestOptions { Namespace = "sb-test", QueueName = QueueName };
        var queue = new ServiceBusPollTriggerQueue(rest, options);
        var scheduler = new PollNextRunScheduler(new PollNextRunSchedulerOptions(), new ZeroJitter());
        var bootstrapper = new PollTriggerBootstrapper(
            queue,
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

        // No existing message to settle.
        await Assert.That(rest.CompletedLockUrls).HasCount().EqualTo(0);
        await Assert.That(rest.AbandonedLockUrls).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_AbandonAndSkipPublish_When_QueueAlreadyHasMessage()
    {
        // Arrange — queue returns a message on the probe receive, meaning the
        // poll-sb cycle is alive. Bootstrapper must NOT publish a duplicate.
        var rest = new FakeServiceBusRestClient();
        var lockUrl = new Uri("https://sb-test.servicebus.windows.net/poll/messages/m1/lock-1");
        rest.EnqueueReceive(new ReceivedServiceBusMessage
        {
            Body = [],
            LockUrl = lockUrl,
        });

        var options = new ServiceBusRestOptions { Namespace = "sb-test", QueueName = QueueName };
        var queue = new ServiceBusPollTriggerQueue(rest, options);
        var scheduler = new PollNextRunScheduler(new PollNextRunSchedulerOptions(), new ZeroJitter());
        var bootstrapper = new PollTriggerBootstrapper(
            queue,
            scheduler,
            new FakeTimeProvider(Now),
            NullLogger<PollTriggerBootstrapper>.Instance);

        // Act
        var result = await bootstrapper.TryBootstrapAsync(CancellationToken.None);

        // Assert — probe succeeded, no publish, message abandoned so the real
        // consumer can redeliver.
        await Assert.That(result.Published).IsFalse();
        await Assert.That(result.ProbeFailed).IsFalse();
        await Assert.That(rest.PublishCalls).HasCount().EqualTo(0);
        await Assert.That(rest.AbandonedLockUrls).HasCount().EqualTo(1);
        await Assert.That(rest.AbandonedLockUrls[0]).IsEqualTo(lockUrl);
        await Assert.That(rest.CompletedLockUrls).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_ReturnFailure_WithoutThrowing_When_TransportFails()
    {
        // Arrange — REST client throws on receive, simulating a transport
        // failure. The safety-net's primary job (polling) must not be aborted.
        var rest = new FakeServiceBusRestClient();
        rest.EnqueueReceiveThrow(new HttpRequestException("sb unreachable"));

        var options = new ServiceBusRestOptions { Namespace = "sb-test", QueueName = QueueName };
        var queue = new ServiceBusPollTriggerQueue(rest, options);
        var scheduler = new PollNextRunScheduler(new PollNextRunSchedulerOptions(), new ZeroJitter());
        var bootstrapper = new PollTriggerBootstrapper(
            queue,
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
