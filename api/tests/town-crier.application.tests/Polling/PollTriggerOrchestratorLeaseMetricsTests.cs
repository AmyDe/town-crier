using System.Diagnostics.Metrics;
using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.Observability;
using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

/// <summary>
/// Verifies that the orchestrator emits lease telemetry counters on the
/// towncrier.polling meter during a happy-path run and on a held-lease run.
/// </summary>
[NotInParallel]
public sealed class PollTriggerOrchestratorLeaseMetricsTests
{
    private static readonly DateTimeOffset Now = new(2026, 4, 24, 12, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task Should_EmitLeaseAcquired_When_HappyPathRun()
    {
        // Arrange
        var leaseStore = new FakePollingLeaseStore();
        var triggerQueue = new FakePollTriggerQueue();
        triggerQueue.EnqueueReceivable();
        var handler = new SpyHandler { NextTerminationReason = PollTerminationReason.Natural };

        var orchestrator = BuildOrchestrator(triggerQueue, handler, leaseStore);

        var acquired = new List<long>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, l) =>
        {
            if (instrument.Name == "towncrier.polling.lease.acquired")
            {
                l.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((_, measurement, _, _) =>
        {
            acquired.Add(measurement);
        });
        listener.Start();

        // Act
        await orchestrator.RunOnceAsync(CancellationToken.None);

        // Assert — exactly one LeaseAcquired emission
        await Assert.That(acquired).HasCount().EqualTo(1);
        await Assert.That(acquired[0]).IsEqualTo(1);
    }

    [Test]
    public async Task Should_EmitLeaseHeldByPeer_When_LeaseUnavailableAfterRetry()
    {
        // Arrange
        var leaseStore = new FakePollingLeaseStore { SimulateHeld = true };
        var triggerQueue = new FakePollTriggerQueue();
        triggerQueue.EnqueueReceivable();

        var orchestrator = BuildOrchestrator(triggerQueue, new SpyHandler(), leaseStore);

        var heldByPeer = new List<long>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, l) =>
        {
            if (instrument.Name == "towncrier.polling.lease.held_by_peer")
            {
                l.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((_, measurement, _, _) =>
        {
            heldByPeer.Add(measurement);
        });
        listener.Start();

        // Act
        var result = await orchestrator.RunOnceAsync(CancellationToken.None);

        // Assert
        await Assert.That(result.LeaseUnavailable).IsTrue();
        await Assert.That(heldByPeer).HasCount().EqualTo(1);
        await Assert.That(heldByPeer[0]).IsEqualTo(1);
    }

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

    private sealed class ZeroJitter : IPollJitter
    {
        public TimeSpan NextOffset(TimeSpan bound) => TimeSpan.Zero;
    }
}
