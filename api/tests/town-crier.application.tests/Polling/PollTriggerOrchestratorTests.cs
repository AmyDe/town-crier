using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

/// <summary>
/// Orchestrator behaviour under ADR 0024 amendment
/// (receive-and-delete + consume-before-publish). The message is destructively
/// consumed on receive — no lock, no Complete, no Abandon. Ordering is
/// receive -> handler -> publish. If anything fails between receive and
/// publish, the chain pauses until the safety-net bootstrap recovers.
/// </summary>
public sealed class PollTriggerOrchestratorTests
{
    private static readonly DateTimeOffset Now = new(2026, 4, 21, 12, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task Should_RunHandlerAndPublishNext_When_TriggerMessageReceived()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);
        var planItClient = new FakePlanItClient();
        planItClient.Add(1, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(1).Build());

        var triggerQueue = new FakePollTriggerQueue();
        triggerQueue.EnqueueReceivable(new FakePollTriggerMessage("M1"));

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);
        var scheduler = new PollNextRunScheduler(new PollNextRunSchedulerOptions(), new ZeroJitter());

        var orchestrator = new PollTriggerOrchestrator(
            handler,
            triggerQueue,
            scheduler,
            new FakeTimeProvider(Now),
            NullLogger<PollTriggerOrchestrator>.Instance);

        var result = await orchestrator.RunOnceAsync(CancellationToken.None);

        await Assert.That(result.MessageReceived).IsTrue();
        await Assert.That(result.PublishedNext).IsTrue();
        await Assert.That(triggerQueue.ScheduledEnqueueTimes).HasCount().EqualTo(1);
        await Assert.That(triggerQueue.ScheduledEnqueueTimes[0]).IsEqualTo(Now + TimeSpan.FromMinutes(5));
    }

    [Test]
    public async Task Should_NotPublish_When_NoTriggerMessageAvailable()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var triggerQueue = new FakePollTriggerQueue();
        var handler = CreateHandler(authorityProvider: authorityProvider);
        var scheduler = new PollNextRunScheduler(new PollNextRunSchedulerOptions(), new ZeroJitter());

        var orchestrator = new PollTriggerOrchestrator(
            handler,
            triggerQueue,
            scheduler,
            new FakeTimeProvider(Now),
            NullLogger<PollTriggerOrchestrator>.Instance);

        var result = await orchestrator.RunOnceAsync(CancellationToken.None);

        await Assert.That(result.MessageReceived).IsFalse();
        await Assert.That(result.PublishedNext).IsFalse();
        await Assert.That(triggerQueue.ScheduledEnqueueTimes).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_NotPublish_When_LeaseIsHeld()
    {
        // LeaseHeld path: another replica holds the Cosmos lease. The trigger
        // message is already destructively consumed (receive-and-delete), so we
        // simply exit — the lease holder is responsible for publishing the
        // next trigger.
        var triggerQueue = new FakePollTriggerQueue();
        triggerQueue.EnqueueReceivable(new FakePollTriggerMessage("M1"));

        var leaseStore = new FakePollingLeaseStore { AcquireResult = false };
        var handler = CreateHandler(leaseStore: leaseStore);
        var scheduler = new PollNextRunScheduler(new PollNextRunSchedulerOptions(), new ZeroJitter());

        var orchestrator = new PollTriggerOrchestrator(
            handler,
            triggerQueue,
            scheduler,
            new FakeTimeProvider(Now),
            NullLogger<PollTriggerOrchestrator>.Instance);

        await orchestrator.RunOnceAsync(CancellationToken.None);

        await Assert.That(triggerQueue.ScheduledEnqueueTimes).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_UseRetryAfter_When_PollingRateLimited()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);
        var planItClient = new FakePlanItClient();
        planItClient.ThrowForAuthority(1, new TownCrier.Application.PlanIt.PlanItRateLimitException(TimeSpan.FromMinutes(2)));

        var triggerQueue = new FakePollTriggerQueue();
        triggerQueue.EnqueueReceivable(new FakePollTriggerMessage("M1"));

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);
        var scheduler = new PollNextRunScheduler(new PollNextRunSchedulerOptions(), new ZeroJitter());

        var orchestrator = new PollTriggerOrchestrator(
            handler,
            triggerQueue,
            scheduler,
            new FakeTimeProvider(Now),
            NullLogger<PollTriggerOrchestrator>.Instance);

        await orchestrator.RunOnceAsync(CancellationToken.None);

        await Assert.That(triggerQueue.ScheduledEnqueueTimes).HasCount().EqualTo(1);
        await Assert.That(triggerQueue.ScheduledEnqueueTimes[0]).IsEqualTo(Now + TimeSpan.FromMinutes(2));
    }

    [Test]
    public async Task Should_PublishNext_When_HandlerReturnsTimeBounded()
    {
        // Force the handler to report TerminationReason=TimeBounded by exhausting
        // the soft HandlerBudget across authority 1's first (and only) page fetch.
        // Spec: docs/specs/poll-handler-soft-budget.md — when the handler returns
        // TimeBounded the orchestrator must still publish-next so the poll-sb
        // chain keeps advancing.
        var start = new DateTimeOffset(2026, 4, 22, 8, 0, 0, TimeSpan.Zero);
        var timeProvider = new FakeTimeProvider(start);

        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);
        authorityProvider.Add(2);

        var planItClient = new FakePlanItClient
        {
            OnFetchComplete = (_, _) => timeProvider.Advance(TimeSpan.FromMinutes(5)),
        };
        planItClient.Add(1, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(1).Build());
        planItClient.Add(2, new PlanningApplicationBuilder().WithUid("app-2").WithAreaId(2).Build());

        var triggerQueue = new FakePollTriggerQueue();
        triggerQueue.EnqueueReceivable(new FakePollTriggerMessage("M1"));

        var handler = CreateHandler(
            planItClient: planItClient,
            authorityProvider: authorityProvider,
            timeProvider: timeProvider,
            options: new PollingOptions { HandlerBudget = TimeSpan.FromMinutes(4) });
        var scheduler = new PollNextRunScheduler(new PollNextRunSchedulerOptions(), new ZeroJitter());

        var orchestrator = new PollTriggerOrchestrator(
            handler,
            triggerQueue,
            scheduler,
            timeProvider,
            NullLogger<PollTriggerOrchestrator>.Instance);

        var runResult = await orchestrator.RunOnceAsync(CancellationToken.None);

        await Assert.That(runResult.MessageReceived).IsTrue();
        await Assert.That(runResult.PublishedNext).IsTrue();
        await Assert.That(runResult.PollResult!.TerminationReason).IsEqualTo(PollTerminationReason.TimeBounded);
        await Assert.That(triggerQueue.ScheduledEnqueueTimes).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_NotPublish_When_HandlerThrows()
    {
        // Under receive-and-delete the message is already gone. If the handler
        // throws, the chain pauses until the safety-net bootstrap recovers —
        // the orchestrator does not attempt to publish a recovery message.
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);
        var planItClient = new FakePlanItClient();
        planItClient.ThrowForAuthority(1, new InvalidOperationException("handler boom"));

        var triggerQueue = new FakePollTriggerQueue();
        triggerQueue.EnqueueReceivable(new FakePollTriggerMessage("M1"));

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);
        var scheduler = new PollNextRunScheduler(new PollNextRunSchedulerOptions(), new ZeroJitter());

        var orchestrator = new PollTriggerOrchestrator(
            handler,
            triggerQueue,
            scheduler,
            new FakeTimeProvider(Now),
            NullLogger<PollTriggerOrchestrator>.Instance);

        await Assert.ThrowsAsync<InvalidOperationException>(
            async () => await orchestrator.RunOnceAsync(CancellationToken.None));

        await Assert.That(triggerQueue.ScheduledEnqueueTimes).HasCount().EqualTo(0);
    }

    private static PollPlanItCommandHandler CreateHandler(
        FakePlanItClient? planItClient = null,
        FakeActiveAuthorityProvider? authorityProvider = null,
        FakePollingLeaseStore? leaseStore = null,
        TimeProvider? timeProvider = null,
        PollingOptions? options = null)
    {
        return new PollPlanItCommandHandler(
            planItClient ?? new FakePlanItClient(),
            new FakePollStateStore(),
            new FakePlanningApplicationRepository(),
            timeProvider ?? TimeProvider.System,
            authorityProvider ?? new FakeActiveAuthorityProvider(),
            new FakeWatchZoneRepository(),
            new FakeNotificationEnqueuer(),
            new FakeCycleSelector(CycleType.Watched),
            options ?? new PollingOptions(),
            leaseStore ?? new FakePollingLeaseStore { AcquireResult = true },
            NullLogger<PollPlanItCommandHandler>.Instance);
    }

    private sealed class ZeroJitter : IPollJitter
    {
        public TimeSpan NextOffset(TimeSpan bound) => TimeSpan.Zero;
    }
}
