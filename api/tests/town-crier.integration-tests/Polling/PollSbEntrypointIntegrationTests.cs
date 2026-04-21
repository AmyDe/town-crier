using System.Diagnostics.CodeAnalysis;
using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.PlanIt;
using TownCrier.Application.Polling;
using TownCrier.Application.Tests.Polling;
using TownCrier.Infrastructure.Polling;
using TownCrier.Infrastructure.ServiceBus;
using TownCrier.Infrastructure.Tests.Polling;

namespace TownCrier.IntegrationTests.Polling;

/// <summary>
/// End-to-end wiring test for the Service-Bus-triggered poll entrypoint.
/// Exercises the REAL <see cref="ServiceBusPollTriggerQueue"/> adapter and the
/// REAL <see cref="PollTriggerOrchestrator"/> and <see cref="PollPlanItCommandHandler"/>,
/// backed by a fake <see cref="IServiceBusRestClient"/> at the transport boundary.
/// The application-layer handler dependencies reuse the in-memory fakes from
/// <c>town-crier.application.tests</c> via linked compilation (see csproj).
/// </summary>
[SuppressMessage(
    "Minor Code Smell",
    "S1075:URIs should not be hardcoded",
    Justification = "Test fixture URIs.")]
public sealed class PollSbEntrypointIntegrationTests
{
    private const string QueueName = "poll";
    private static readonly DateTimeOffset Now = new(2026, 4, 21, 12, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task Should_PublishNextMessage_BeforeAck_When_HandlerSucceeds()
    {
        // Arrange — real adapter, real orchestrator, real handler, fake REST client.
        var rest = new FakeServiceBusRestClient();
        var lockUrl = new Uri("https://sb-test.servicebus.windows.net/poll/messages/m1/lock-1");
        rest.EnqueueReceive(new ReceivedServiceBusMessage
        {
            Body = [],
            LockUrl = lockUrl,
        });

        var options = new ServiceBusRestOptions { Namespace = "sb-test", QueueName = QueueName };
        var queue = new ServiceBusPollTriggerQueue(rest, options);

        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);
        var planItClient = new FakePlanItClient();
        planItClient.Add(1, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(1).Build());

        var handler = CreateHandler(planItClient, authorityProvider);
        var scheduler = new PollNextRunScheduler(new PollNextRunSchedulerOptions(), new ZeroJitter());
        var orchestrator = new PollTriggerOrchestrator(
            handler,
            queue,
            scheduler,
            new FakeTimeProvider(Now),
            NullLogger<PollTriggerOrchestrator>.Instance);

        // Act
        var result = await orchestrator.RunOnceAsync(CancellationToken.None);

        // Assert — message received, handler ran, publish-before-ack observed.
        await Assert.That(result.MessageReceived).IsTrue();
        await Assert.That(result.PublishedNext).IsTrue();
        await Assert.That(rest.PublishCalls).HasCount().EqualTo(1);
        await Assert.That(rest.CompletedLockUrls).HasCount().EqualTo(1);
        await Assert.That(rest.CompletedLockUrls[0]).IsEqualTo(lockUrl);

        // ScheduledEnqueueTimeUtc equals now + natural cadence (5 min default).
        await Assert.That(rest.PublishCalls[0].ScheduledEnqueueTimeUtc)
            .IsEqualTo(Now + TimeSpan.FromMinutes(5));

        // Publish-before-ack ordering is load-bearing for crash safety.
        await Assert.That(rest.CallSequence.IndexOf("publish"))
            .IsLessThan(rest.CallSequence.IndexOf("complete"));
    }

    [Test]
    public async Task Should_PublishWithRetryAfter_When_HandlerHitsRateLimit()
    {
        // Arrange — handler throws PlanItRateLimitException; handler converts that into a
        // RateLimited termination with RetryAfter bubbled up, which the scheduler honours.
        var rest = new FakeServiceBusRestClient();
        var lockUrl = new Uri("https://sb-test.servicebus.windows.net/poll/messages/m2/lock-2");
        rest.EnqueueReceive(new ReceivedServiceBusMessage
        {
            Body = [],
            LockUrl = lockUrl,
        });

        var options = new ServiceBusRestOptions { Namespace = "sb-test", QueueName = QueueName };
        var queue = new ServiceBusPollTriggerQueue(rest, options);

        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);
        var planItClient = new FakePlanItClient();
        planItClient.ThrowForAuthority(1, new PlanItRateLimitException(TimeSpan.FromMinutes(2)));

        var handler = CreateHandler(planItClient, authorityProvider);
        var scheduler = new PollNextRunScheduler(new PollNextRunSchedulerOptions(), new ZeroJitter());
        var orchestrator = new PollTriggerOrchestrator(
            handler,
            queue,
            scheduler,
            new FakeTimeProvider(Now),
            NullLogger<PollTriggerOrchestrator>.Instance);

        // Act
        var result = await orchestrator.RunOnceAsync(CancellationToken.None);

        // Assert — publish scheduled at now + Retry-After, message acked.
        await Assert.That(result.MessageReceived).IsTrue();
        await Assert.That(result.PublishedNext).IsTrue();
        await Assert.That(rest.PublishCalls).HasCount().EqualTo(1);
        await Assert.That(rest.PublishCalls[0].ScheduledEnqueueTimeUtc)
            .IsEqualTo(Now + TimeSpan.FromMinutes(2));
        await Assert.That(rest.CompletedLockUrls).HasCount().EqualTo(1);
        await Assert.That(rest.CompletedLockUrls[0]).IsEqualTo(lockUrl);
    }

    [Test]
    public async Task Should_NoOp_When_QueueIsEmpty()
    {
        // Arrange — no message enqueued, REST client returns null.
        var rest = new FakeServiceBusRestClient();
        var options = new ServiceBusRestOptions { Namespace = "sb-test", QueueName = QueueName };
        var queue = new ServiceBusPollTriggerQueue(rest, options);

        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);
        var handler = CreateHandler(new FakePlanItClient(), authorityProvider);
        var scheduler = new PollNextRunScheduler(new PollNextRunSchedulerOptions(), new ZeroJitter());
        var orchestrator = new PollTriggerOrchestrator(
            handler,
            queue,
            scheduler,
            new FakeTimeProvider(Now),
            NullLogger<PollTriggerOrchestrator>.Instance);

        // Act
        var result = await orchestrator.RunOnceAsync(CancellationToken.None);

        // Assert — no handler run, no publish, no complete.
        await Assert.That(result.MessageReceived).IsFalse();
        await Assert.That(result.PublishedNext).IsFalse();
        await Assert.That(rest.PublishCalls).HasCount().EqualTo(0);
        await Assert.That(rest.CompletedLockUrls).HasCount().EqualTo(0);
        await Assert.That(rest.AbandonedLockUrls).HasCount().EqualTo(0);
    }

    private static PollPlanItCommandHandler CreateHandler(
        FakePlanItClient planItClient,
        FakeActiveAuthorityProvider authorityProvider)
    {
        return new PollPlanItCommandHandler(
            planItClient,
            new FakePollStateStore(),
            new FakePlanningApplicationRepository(),
            TimeProvider.System,
            authorityProvider,
            new FakeWatchZoneRepository(),
            new FakeNotificationEnqueuer(),
            new FakeCycleSelector(CycleType.Watched),
            new PollingOptions(),
            new FakePollingLeaseStore { AcquireResult = true },
            NullLogger<PollPlanItCommandHandler>.Instance);
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
