using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

public sealed class PollTriggerOrchestratorTests
{
    private static readonly DateTimeOffset Now = new(2026, 4, 21, 12, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task Should_PublishNextMessage_Before_CompletingTriggerMessage()
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

        await orchestrator.RunOnceAsync(CancellationToken.None);

        await Assert.That(triggerQueue.ScheduleSequence).HasCount().EqualTo(2);
        await Assert.That(triggerQueue.ScheduleSequence[0]).IsEqualTo("publish");
        await Assert.That(triggerQueue.ScheduleSequence[1]).IsEqualTo("complete");
    }

    [Test]
    public async Task Should_PublishNextMessage_When_TriggerMessageReceived()
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

        await orchestrator.RunOnceAsync(CancellationToken.None);

        await Assert.That(triggerQueue.ScheduledEnqueueTimes).HasCount().EqualTo(1);
        await Assert.That(triggerQueue.ScheduledEnqueueTimes[0]).IsEqualTo(Now + TimeSpan.FromMinutes(5));
    }

    [Test]
    public async Task Should_NotPublishOrComplete_When_NoTriggerMessageAvailable()
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

        await orchestrator.RunOnceAsync(CancellationToken.None);

        await Assert.That(triggerQueue.ScheduledEnqueueTimes).HasCount().EqualTo(0);
        await Assert.That(triggerQueue.CompletedCount).IsEqualTo(0);
    }

    [Test]
    public async Task Should_NotPublish_When_LeaseIsHeld()
    {
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

        // Abandon the message so the current holder's publish remains the authoritative
        // continuation of the chain.
        await Assert.That(triggerQueue.AbandonedCount).IsEqualTo(1);
        await Assert.That(triggerQueue.CompletedCount).IsEqualTo(0);
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

    private static PollPlanItCommandHandler CreateHandler(
        FakePlanItClient? planItClient = null,
        FakeActiveAuthorityProvider? authorityProvider = null,
        FakePollingLeaseStore? leaseStore = null)
    {
        return new PollPlanItCommandHandler(
            planItClient ?? new FakePlanItClient(),
            new FakePollStateStore(),
            new FakePlanningApplicationRepository(),
            TimeProvider.System,
            authorityProvider ?? new FakeActiveAuthorityProvider(),
            new FakeWatchZoneRepository(),
            new FakeNotificationEnqueuer(),
            new FakeCycleSelector(CycleType.Watched),
            new PollingOptions(),
            leaseStore ?? new FakePollingLeaseStore { AcquireResult = true },
            NullLogger<PollPlanItCommandHandler>.Instance);
    }

    private sealed class ZeroJitter : IPollJitter
    {
        public TimeSpan NextOffset(TimeSpan bound) => TimeSpan.Zero;
    }
}
