using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

/// <summary>
/// Orchestrator behaviour under ADR 0024 amendment and the polling-lease-CAS spec.
/// The message is destructively consumed on receive — no lock, no Complete, no Abandon.
/// Ordering is: acquire lease -> receive -> handler -> publish -> release.
/// If anything fails between acquire and release, the lease is still released in finally.
/// </summary>
public sealed class PollTriggerOrchestratorTests
{
    private static readonly DateTimeOffset Now = new(2026, 4, 21, 12, 0, 0, TimeSpan.Zero);

    // ---------------------------------------------------------------------------
    // Lease-gating tests (new in T11)
    // ---------------------------------------------------------------------------
    [Test]
    public async Task Should_ReturnLeaseUnavailable_When_LeaseHeldAcrossBothAttempts()
    {
        var leaseStore = new FakePollingLeaseStore { SimulateHeld = true };
        var triggerQueue = new FakePollTriggerQueue();
        triggerQueue.EnqueueReceivable();
        var handler = new SpyHandler();

        var orchestrator = BuildOrchestrator(triggerQueue, handler, leaseStore);

        var result = await orchestrator.RunOnceAsync(default);

        await Assert.That(result.LeaseUnavailable).IsTrue();
        await Assert.That(result.MessageReceived).IsFalse();
        await Assert.That(handler.HandleCalls).IsEqualTo(0);
        await Assert.That(triggerQueue.ReceiveCalls).IsEqualTo(0);
        await Assert.That(leaseStore.AcquireCalls).IsEqualTo(2); // one retry
    }

    [Test]
    public async Task Should_ProceedAfterRetry_When_LeaseHeldOnFirstAttemptOnly()
    {
        var leaseStore = new FakePollingLeaseStore { SimulateHeld = true };
        var triggerQueue = new FakePollTriggerQueue();
        triggerQueue.EnqueueReceivable();
        var handler = new SpyHandler { NextTerminationReason = PollTerminationReason.Natural };

        // First call returns Held; second call returns Acquired.
        var gatedLease = new OneShotGatedLeaseStore(leaseStore);

        var orchestrator = BuildOrchestrator(triggerQueue, handler, gatedLease);

        var result = await orchestrator.RunOnceAsync(default);

        await Assert.That(result.LeaseUnavailable).IsFalse();
        await Assert.That(result.MessageReceived).IsTrue();
        await Assert.That(result.PublishedNext).IsTrue();
        await Assert.That(handler.HandleCalls).IsEqualTo(1);
        await Assert.That(leaseStore.ReleaseCalls).IsEqualTo(1);
    }

    [Test]
    public async Task Should_ReleaseLease_When_HandlerThrows()
    {
        var leaseStore = new FakePollingLeaseStore();
        var triggerQueue = new FakePollTriggerQueue();
        triggerQueue.EnqueueReceivable();
        var handler = new SpyHandler { ThrowsOnHandle = new InvalidOperationException("bang") };

        var orchestrator = BuildOrchestrator(triggerQueue, handler, leaseStore);

        await Assert.ThrowsAsync<InvalidOperationException>(async () =>
            await orchestrator.RunOnceAsync(default));

        await Assert.That(leaseStore.ReleaseCalls).IsEqualTo(1);
    }

    [Test]
    public async Task Should_ReleaseLease_When_PublishThrows()
    {
        var leaseStore = new FakePollingLeaseStore();
        var triggerQueue = new FakePollTriggerQueue { ThrowOnPublish = new InvalidOperationException("bang") };
        triggerQueue.EnqueueReceivable();
        var handler = new SpyHandler { NextTerminationReason = PollTerminationReason.Natural };

        var orchestrator = BuildOrchestrator(triggerQueue, handler, leaseStore);

        await Assert.ThrowsAsync<InvalidOperationException>(async () =>
            await orchestrator.RunOnceAsync(default));

        await Assert.That(leaseStore.ReleaseCalls).IsEqualTo(1);
    }

    // ---------------------------------------------------------------------------
    // Existing end-to-end tests (updated to pass lease store + options)
    // ---------------------------------------------------------------------------
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
            new FakePollingLeaseStore(),
            new PollingOptions { LeaseAcquireRetryDelay = TimeSpan.Zero },
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
            new FakePollingLeaseStore(),
            new PollingOptions { LeaseAcquireRetryDelay = TimeSpan.Zero },
            new FakeTimeProvider(Now),
            NullLogger<PollTriggerOrchestrator>.Instance);

        var result = await orchestrator.RunOnceAsync(CancellationToken.None);

        await Assert.That(result.MessageReceived).IsFalse();
        await Assert.That(result.PublishedNext).IsFalse();
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
            new FakePollingLeaseStore(),
            new PollingOptions { LeaseAcquireRetryDelay = TimeSpan.Zero },
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
            new FakePollingLeaseStore(),
            new PollingOptions { LeaseAcquireRetryDelay = TimeSpan.Zero },
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
        // fails at a level outside its per-authority error handling, for
        // example the active-authority provider itself faults, the chain pauses
        // until the safety-net bootstrap recovers and the orchestrator does
        // not attempt to publish a recovery message.
        var authorityProvider = new ThrowingActiveAuthorityProvider(
            new InvalidOperationException("handler boom"));

        var triggerQueue = new FakePollTriggerQueue();
        triggerQueue.EnqueueReceivable(new FakePollTriggerMessage("M1"));

        var handler = new PollPlanItCommandHandler(
            new FakePlanItClient(),
            new FakePollStateStore(),
            new FakePlanningApplicationRepository(),
            TimeProvider.System,
            authorityProvider,
            new FakeWatchZoneRepository(),
            new FakeNotificationEnqueuer(),
            new FakeCycleSelector(CycleType.Watched),
            new PollingOptions(),
            NullLogger<PollPlanItCommandHandler>.Instance);
        var scheduler = new PollNextRunScheduler(new PollNextRunSchedulerOptions(), new ZeroJitter());

        var orchestrator = new PollTriggerOrchestrator(
            handler,
            triggerQueue,
            scheduler,
            new FakePollingLeaseStore(),
            new PollingOptions { LeaseAcquireRetryDelay = TimeSpan.Zero },
            new FakeTimeProvider(Now),
            NullLogger<PollTriggerOrchestrator>.Instance);

        await Assert.ThrowsAsync<InvalidOperationException>(
            async () => await orchestrator.RunOnceAsync(CancellationToken.None));

        await Assert.That(triggerQueue.ScheduledEnqueueTimes).HasCount().EqualTo(0);
    }

    // ---------------------------------------------------------------------------
    // Helpers
    // ---------------------------------------------------------------------------
    private static PollTriggerOrchestrator BuildOrchestrator(
        FakePollTriggerQueue triggerQueue,
        IPollPlanItCommandHandler handler,
        IPollingLeaseStore leaseStore)
    {
        return new PollTriggerOrchestrator(
            handler,
            triggerQueue,
            new PollNextRunScheduler(new PollNextRunSchedulerOptions(), new ZeroJitter()),
            leaseStore,
            new PollingOptions { LeaseAcquireRetryDelay = TimeSpan.Zero },
            new FakeTimeProvider(Now),
            NullLogger<PollTriggerOrchestrator>.Instance);
    }

    private static PollPlanItCommandHandler CreateHandler(
        FakePlanItClient? planItClient = null,
        FakeActiveAuthorityProvider? authorityProvider = null,
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
            NullLogger<PollPlanItCommandHandler>.Instance);
    }

    private sealed class ZeroJitter : IPollJitter
    {
        public TimeSpan NextOffset(TimeSpan bound) => TimeSpan.Zero;
    }

    private sealed class ThrowingActiveAuthorityProvider : IActiveAuthorityProvider
    {
        private readonly Exception exception;

        public ThrowingActiveAuthorityProvider(Exception exception)
        {
            this.exception = exception;
        }

        public Task<IReadOnlyCollection<int>> GetActiveAuthorityIdsAsync(CancellationToken ct)
        {
            throw this.exception;
        }
    }

    private sealed class OneShotGatedLeaseStore(FakePollingLeaseStore inner) : IPollingLeaseStore
    {
        private int calls;

        public Task<LeaseAcquireResult> TryAcquireAsync(TimeSpan ttl, CancellationToken ct)
        {
            var n = Interlocked.Increment(ref this.calls);
            if (n == 1)
            {
                inner.SimulateHeld = true;
                return inner.TryAcquireAsync(ttl, ct);
            }

            inner.SimulateHeld = false;
            return inner.TryAcquireAsync(ttl, ct);
        }

        public Task<LeaseReleaseOutcome> ReleaseAsync(LeaseHandle handle, CancellationToken ct) =>
            inner.ReleaseAsync(handle, ct);
    }
}
