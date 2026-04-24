using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

/// <summary>
/// Safety-net re-seed behaviour under ADR 0024 amendment
/// (receive-and-delete + management-API probe). The safety-net cron path now
/// runs only the bootstrapper, which reads activeMessageCount +
/// scheduledMessageCount via the management API and seeds a bootstrap trigger
/// iff both are zero.
/// </summary>
public sealed class PollTriggerBootstrapperTests
{
    private static readonly DateTimeOffset Now = new(2026, 4, 21, 12, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task Should_ReturnLeaseUnavailable_When_LeaseHeld()
    {
        var leaseStore = new FakePollingLeaseStore { SimulateHeld = true };
        var triggerQueue = new FakePollTriggerQueue();
        var metrics = new FakePollTriggerQueueMetrics();

        var bootstrapper = BuildBootstrapper(triggerQueue, metrics, leaseStore);

        var result = await bootstrapper.TryBootstrapAsync(default);

        await Assert.That(result.LeaseUnavailable).IsTrue();
        await Assert.That(result.Published).IsFalse();
        await Assert.That(metrics.ProbeCalls).IsEqualTo(0);
        await Assert.That(triggerQueue.PublishCalls).IsEqualTo(0);
    }

    [Test]
    public async Task Should_PublishAndRelease_When_LeaseAcquiredAndQueueEmpty()
    {
        var leaseStore = new FakePollingLeaseStore();
        var triggerQueue = new FakePollTriggerQueue();
        var metrics = new FakePollTriggerQueueMetrics();
        metrics.Enqueue(active: 0, scheduled: 0);

        var bootstrapper = BuildBootstrapper(triggerQueue, metrics, leaseStore);

        var result = await bootstrapper.TryBootstrapAsync(default);

        await Assert.That(result.Published).IsTrue();
        await Assert.That(result.LeaseUnavailable).IsFalse();
        await Assert.That(triggerQueue.PublishCalls).IsEqualTo(1);
        await Assert.That(leaseStore.ReleaseCalls).IsEqualTo(1);
    }

    [Test]
    public async Task Should_ReleaseLease_When_ProbeThrows()
    {
        var leaseStore = new FakePollingLeaseStore();
        var triggerQueue = new FakePollTriggerQueue();
        var metrics = new FakePollTriggerQueueMetrics { ThrowOnProbe = new InvalidOperationException("boom") };

        var bootstrapper = BuildBootstrapper(triggerQueue, metrics, leaseStore);

        var result = await bootstrapper.TryBootstrapAsync(default);

        await Assert.That(result.ProbeFailed).IsTrue();
        await Assert.That(leaseStore.ReleaseCalls).IsEqualTo(1);
    }

    [Test]
    public async Task Should_PublishBootstrapTrigger_When_BothCountsAreZero()
    {
        var triggerQueue = new FakePollTriggerQueue();
        var metrics = new FakePollTriggerQueueMetrics();
        metrics.Enqueue(active: 0, scheduled: 0);

        var bootstrapper = BuildBootstrapper(triggerQueue, metrics);

        var result = await bootstrapper.TryBootstrapAsync(CancellationToken.None);

        await Assert.That(result.Published).IsTrue();
        await Assert.That(result.ProbeFailed).IsFalse();
        await Assert.That(triggerQueue.ScheduledEnqueueTimes).HasCount().EqualTo(1);

        // Natural cadence (5 min default) applied by PollNextRunScheduler so the
        // adaptive/jittered schedule is used even for the bootstrap.
        await Assert.That(triggerQueue.ScheduledEnqueueTimes[0])
            .IsEqualTo(Now + TimeSpan.FromMinutes(5));
    }

    [Test]
    public async Task Should_NotPublish_When_ActiveCountPositive()
    {
        var triggerQueue = new FakePollTriggerQueue();
        var metrics = new FakePollTriggerQueueMetrics();
        metrics.Enqueue(active: 1, scheduled: 0);

        var bootstrapper = BuildBootstrapper(triggerQueue, metrics);

        var result = await bootstrapper.TryBootstrapAsync(CancellationToken.None);

        await Assert.That(result.Published).IsFalse();
        await Assert.That(result.ProbeFailed).IsFalse();
        await Assert.That(triggerQueue.ScheduledEnqueueTimes).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_NotPublish_When_ScheduledCountPositive()
    {
        // The management-API probe closes the pre-existing blind spot where the
        // old PeekLock probe could not see future-dated messages and would
        // double-publish when a healthy chain was paused on Retry-After.
        var triggerQueue = new FakePollTriggerQueue();
        var metrics = new FakePollTriggerQueueMetrics();
        metrics.Enqueue(active: 0, scheduled: 1);

        var bootstrapper = BuildBootstrapper(triggerQueue, metrics);

        var result = await bootstrapper.TryBootstrapAsync(CancellationToken.None);

        await Assert.That(result.Published).IsFalse();
        await Assert.That(result.ProbeFailed).IsFalse();
        await Assert.That(triggerQueue.ScheduledEnqueueTimes).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_NotPublish_When_BothCountsPositive()
    {
        var triggerQueue = new FakePollTriggerQueue();
        var metrics = new FakePollTriggerQueueMetrics();
        metrics.Enqueue(active: 2, scheduled: 3);

        var bootstrapper = BuildBootstrapper(triggerQueue, metrics);

        var result = await bootstrapper.TryBootstrapAsync(CancellationToken.None);

        await Assert.That(result.Published).IsFalse();
        await Assert.That(result.ProbeFailed).IsFalse();
        await Assert.That(triggerQueue.ScheduledEnqueueTimes).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_ReturnFailure_When_MetricsProbeThrows()
    {
        // A failure to probe the queue must not throw — the bootstrapper is
        // best-effort and a later safety-net tick will retry.
        var triggerQueue = new FakePollTriggerQueue();
        var metrics = new FakePollTriggerQueueMetrics();
        metrics.EnqueueThrow(new InvalidOperationException("management-api unreachable"));

        var bootstrapper = BuildBootstrapper(triggerQueue, metrics);

        var result = await bootstrapper.TryBootstrapAsync(CancellationToken.None);

        await Assert.That(result.Published).IsFalse();
        await Assert.That(result.ProbeFailed).IsTrue();
        await Assert.That(triggerQueue.ScheduledEnqueueTimes).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_ReturnFailure_When_PublishThrows()
    {
        // Publish failures are also absorbed — a later safety-net tick will
        // retry, and the chain continues to run on its own pulse in the
        // meantime.
        var triggerQueue = new FakePollTriggerQueue { ThrowOnPublish = new InvalidOperationException("publish failed") };
        var metrics = new FakePollTriggerQueueMetrics();
        metrics.Enqueue(active: 0, scheduled: 0);

        var bootstrapper = BuildBootstrapper(triggerQueue, metrics);

        var result = await bootstrapper.TryBootstrapAsync(CancellationToken.None);

        await Assert.That(result.Published).IsFalse();
        await Assert.That(result.ProbeFailed).IsTrue();
    }

    private static PollTriggerBootstrapper BuildBootstrapper(
        IPollTriggerQueue triggerQueue,
        FakePollTriggerQueueMetrics metrics,
        FakePollingLeaseStore? leaseStore = null)
    {
        return new PollTriggerBootstrapper(
            triggerQueue,
            metrics,
            new PollNextRunScheduler(new PollNextRunSchedulerOptions(), new ZeroJitter()),
            leaseStore ?? new FakePollingLeaseStore(),
            new PollingOptions(),
            new FakeTimeProvider(Now),
            NullLogger<PollTriggerBootstrapper>.Instance);
    }

    private sealed class ZeroJitter : IPollJitter
    {
        public TimeSpan NextOffset(TimeSpan bound) => TimeSpan.Zero;
    }
}
