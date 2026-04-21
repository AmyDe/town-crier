using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

/// <summary>
/// Safety-net re-seed behaviour (see bd tc-tdgf). The safety-net cron path
/// polls PlanIt directly; afterwards, the bootstrapper checks the Service Bus
/// poll trigger queue and publishes a jittered bootstrap trigger iff the queue
/// is empty, so the adaptive SB-coordinated cycle self-heals.
/// </summary>
public sealed class PollTriggerBootstrapperTests
{
    private static readonly DateTimeOffset Now = new(2026, 4, 21, 12, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task Should_PublishBootstrapTrigger_When_QueueIsEmpty()
    {
        var triggerQueue = new FakePollTriggerQueue();
        var scheduler = new PollNextRunScheduler(new PollNextRunSchedulerOptions(), new ZeroJitter());
        var bootstrapper = new PollTriggerBootstrapper(
            triggerQueue,
            scheduler,
            new FakeTimeProvider(Now),
            NullLogger<PollTriggerBootstrapper>.Instance);

        var result = await bootstrapper.TryBootstrapAsync(CancellationToken.None);

        await Assert.That(result.Published).IsTrue();
        await Assert.That(triggerQueue.ScheduledEnqueueTimes).HasCount().EqualTo(1);

        // Natural cadence (5 min default) applied by PollNextRunScheduler so the
        // adaptive/jittered schedule is used even for the bootstrap.
        await Assert.That(triggerQueue.ScheduledEnqueueTimes[0])
            .IsEqualTo(Now + TimeSpan.FromMinutes(5));
    }

    [Test]
    public async Task Should_NotPublish_When_QueueAlreadyHasMessage()
    {
        var triggerQueue = new FakePollTriggerQueue();
        triggerQueue.EnqueueReceivable(new FakePollTriggerMessage("existing"));

        var scheduler = new PollNextRunScheduler(new PollNextRunSchedulerOptions(), new ZeroJitter());
        var bootstrapper = new PollTriggerBootstrapper(
            triggerQueue,
            scheduler,
            new FakeTimeProvider(Now),
            NullLogger<PollTriggerBootstrapper>.Instance);

        var result = await bootstrapper.TryBootstrapAsync(CancellationToken.None);

        await Assert.That(result.Published).IsFalse();
        await Assert.That(triggerQueue.ScheduledEnqueueTimes).HasCount().EqualTo(0);

        // Abandon the PeekLock so the real poll-sb consumer can redeliver and
        // settle the message.
        await Assert.That(triggerQueue.AbandonedCount).IsEqualTo(1);
        await Assert.That(triggerQueue.CompletedCount).IsEqualTo(0);
    }

    [Test]
    public async Task Should_ReturnFailure_When_QueueCheckThrows()
    {
        // A failure to probe the queue must not throw — the safety-net's
        // primary job is to poll, reseeding is best-effort.
        var triggerQueue = new ThrowingPollTriggerQueue();
        var scheduler = new PollNextRunScheduler(new PollNextRunSchedulerOptions(), new ZeroJitter());
        var bootstrapper = new PollTriggerBootstrapper(
            triggerQueue,
            scheduler,
            new FakeTimeProvider(Now),
            NullLogger<PollTriggerBootstrapper>.Instance);

        var result = await bootstrapper.TryBootstrapAsync(CancellationToken.None);

        await Assert.That(result.Published).IsFalse();
        await Assert.That(result.ProbeFailed).IsTrue();
    }

    [Test]
    public async Task Should_ReturnFailure_When_PublishThrows()
    {
        // Publish failures are also absorbed — a later safety-net tick will
        // retry, and in the meantime the cron keeps polling.
        var triggerQueue = new PublishThrowingPollTriggerQueue();
        var scheduler = new PollNextRunScheduler(new PollNextRunSchedulerOptions(), new ZeroJitter());
        var bootstrapper = new PollTriggerBootstrapper(
            triggerQueue,
            scheduler,
            new FakeTimeProvider(Now),
            NullLogger<PollTriggerBootstrapper>.Instance);

        var result = await bootstrapper.TryBootstrapAsync(CancellationToken.None);

        await Assert.That(result.Published).IsFalse();
        await Assert.That(result.ProbeFailed).IsTrue();
    }

    private sealed class ZeroJitter : IPollJitter
    {
        public TimeSpan NextOffset(TimeSpan bound) => TimeSpan.Zero;
    }

    private sealed class ThrowingPollTriggerQueue : IPollTriggerQueue
    {
        public Task<IPollTriggerMessage?> ReceiveAsync(CancellationToken ct)
            => throw new InvalidOperationException("queue unreachable");

        public Task PublishAtAsync(DateTimeOffset scheduledEnqueueTime, CancellationToken ct)
            => Task.CompletedTask;

        public Task CompleteAsync(IPollTriggerMessage message, CancellationToken ct)
            => Task.CompletedTask;

        public Task AbandonAsync(IPollTriggerMessage message, CancellationToken ct)
            => Task.CompletedTask;
    }

    private sealed class PublishThrowingPollTriggerQueue : IPollTriggerQueue
    {
        public Task<IPollTriggerMessage?> ReceiveAsync(CancellationToken ct)
            => Task.FromResult<IPollTriggerMessage?>(null);

        public Task PublishAtAsync(DateTimeOffset scheduledEnqueueTime, CancellationToken ct)
            => throw new InvalidOperationException("publish failed");

        public Task CompleteAsync(IPollTriggerMessage message, CancellationToken ct)
            => Task.CompletedTask;

        public Task AbandonAsync(IPollTriggerMessage message, CancellationToken ct)
            => Task.CompletedTask;
    }
}
