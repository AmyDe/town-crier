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
/// Under ADR 0024 amendment (2026-04-22) the queue runs in receive-and-delete
/// mode — the message is destructively consumed on receive, so there is no
/// lock and no Complete/Abandon. Ordering is receive -> handler -> publish.
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
    public async Task Should_RunHandlerThenPublishNext_When_HandlerSucceeds()
    {
        // Arrange — real adapter, real orchestrator, real handler, fake REST client.
        var rest = new FakeServiceBusRestClient();
        rest.EnqueueReceive(new ReceivedServiceBusMessage
        {
            Body = [],
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
            new FakePollingLeaseStore(),
            new PollingOptions { LeaseAcquireRetryDelay = TimeSpan.Zero },
            new FakeTimeProvider(Now),
            NullLogger<PollTriggerOrchestrator>.Instance);

        // Act
        var result = await orchestrator.RunOnceAsync(CancellationToken.None);

        // Assert — message received, handler ran, next trigger published.
        await Assert.That(result.MessageReceived).IsTrue();
        await Assert.That(result.PublishedNext).IsTrue();
        await Assert.That(rest.PublishCalls).HasCount().EqualTo(1);

        // ScheduledEnqueueTimeUtc equals now + natural cadence (5 min default).
        await Assert.That(rest.PublishCalls[0].ScheduledEnqueueTimeUtc)
            .IsEqualTo(Now + TimeSpan.FromMinutes(5));

        // Receive-before-publish ordering: destructive consume happens first,
        // then the handler runs, then the next trigger is published. Crash
        // safety is recovered by the safety-net bootstrap, not by ack ordering.
        await Assert.That(rest.CallSequence.IndexOf("receive"))
            .IsLessThan(rest.CallSequence.IndexOf("publish"));
    }

    [Test]
    public async Task Should_PublishWithRetryAfter_When_HandlerHitsRateLimit()
    {
        // Arrange — handler throws PlanItRateLimitException; handler converts that into a
        // RateLimited termination with RetryAfter bubbled up, which the scheduler honours.
        var rest = new FakeServiceBusRestClient();
        rest.EnqueueReceive(new ReceivedServiceBusMessage
        {
            Body = [],
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
            new FakePollingLeaseStore(),
            new PollingOptions { LeaseAcquireRetryDelay = TimeSpan.Zero },
            new FakeTimeProvider(Now),
            NullLogger<PollTriggerOrchestrator>.Instance);

        // Act
        var result = await orchestrator.RunOnceAsync(CancellationToken.None);

        // Assert — publish scheduled at now + Retry-After; no ack step (receive-and-delete).
        await Assert.That(result.MessageReceived).IsTrue();
        await Assert.That(result.PublishedNext).IsTrue();
        await Assert.That(rest.PublishCalls).HasCount().EqualTo(1);
        await Assert.That(rest.PublishCalls[0].ScheduledEnqueueTimeUtc)
            .IsEqualTo(Now + TimeSpan.FromMinutes(2));
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
            new FakePollingLeaseStore(),
            new PollingOptions { LeaseAcquireRetryDelay = TimeSpan.Zero },
            new FakeTimeProvider(Now),
            NullLogger<PollTriggerOrchestrator>.Instance);

        // Act
        var result = await orchestrator.RunOnceAsync(CancellationToken.None);

        // Assert — no handler run, no publish.
        await Assert.That(result.MessageReceived).IsFalse();
        await Assert.That(result.PublishedNext).IsFalse();
        await Assert.That(rest.PublishCalls).HasCount().EqualTo(0);
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
