using System.Diagnostics.CodeAnalysis;
using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.Polling;
using TownCrier.Application.Tests.Polling;
using TownCrier.Infrastructure.Cosmos;
using TownCrier.Infrastructure.Polling;
using TownCrier.Infrastructure.Tests.Cosmos;

namespace TownCrier.IntegrationTests.Polling;

/// <summary>
/// Contention simulation: 100 iterations each spawn one <see cref="PollTriggerOrchestrator"/>
/// and one <see cref="PollTriggerBootstrapper"/> concurrently against a shared
/// <see cref="FakePollTriggerQueue"/> backed by a real <see cref="CosmosPollingLeaseStore"/>
/// (using the ETag-CAS-aware <see cref="FakeCosmosRestClient"/>). The CAS lease
/// must guarantee that total publishes per round never exceeds 1.
/// </summary>
public sealed class PollLeaseCasIntegrationTests
{
    [Test]
    public async Task QueueDepth_StaysBoundedAtOne_UnderContention()
    {
        for (var iteration = 0; iteration < 100; iteration++)
        {
            await RunOneContentionRound(iteration);
        }
    }

    [SuppressMessage(
        "Security",
        "CA5394:Do not use insecure randomness",
        Justification = "Seeded Random used for deterministic test ordering only; no security requirement.")]
    private static async Task RunOneContentionRound(int seed)
    {
        var cosmos = new FakeCosmosRestClient();
        var time = new FakeTimeProvider(new DateTimeOffset(2026, 4, 23, 12, 0, 0, TimeSpan.Zero));
        var leaseStore = new CosmosPollingLeaseStore(cosmos, time);

        var queue = new FakePollTriggerQueue();
        queue.EnqueueReceivable();

        var metrics = new FakePollTriggerQueueMetrics();

        // The orchestrator will destructively receive the trigger. The bootstrapper
        // probes via metrics; we enqueue active:1 so the bootstrap sees a non-empty
        // queue if it wins the lease before the orchestrator drains it. In the race
        // case where the orchestrator wins first the metrics call never happens because
        // the bootstrap exits at the lease-unavailable guard.
        metrics.Enqueue(active: 1, scheduled: 0);

        var scheduler = new PollNextRunScheduler(new PollNextRunSchedulerOptions(), new ZeroJitter());
        var handler = new SpyHandler { NextTerminationReason = PollTerminationReason.Natural };
        var options = new PollingOptions
        {
            OrchestratorLeaseTtl = TimeSpan.FromMinutes(5),
            BootstrapLeaseTtl = TimeSpan.FromSeconds(60),
            LeaseAcquireRetryDelay = TimeSpan.FromMilliseconds(5),
        };

        var orchestrator = new PollTriggerOrchestrator(
            handler,
            queue,
            scheduler,
            leaseStore,
            options,
            time,
            NullLogger<PollTriggerOrchestrator>.Instance);
        var bootstrapper = new PollTriggerBootstrapper(
            queue,
            metrics,
            scheduler,
            leaseStore,
            options,
            time,
            NullLogger<PollTriggerBootstrapper>.Instance);

        var rng = new Random(seed);
        var orchestratorFirst = rng.Next(2) == 0;

        Task<PollTriggerOrchestratorRunResult> orchTask;
        Task<PollTriggerBootstrapResult> bootTask;

        if (orchestratorFirst)
        {
            orchTask = Task.Run(() => orchestrator.RunOnceAsync(default));
            await Task.Delay(rng.Next(5), default);
            bootTask = Task.Run(() => bootstrapper.TryBootstrapAsync(default));
        }
        else
        {
            bootTask = Task.Run(() => bootstrapper.TryBootstrapAsync(default));
            await Task.Delay(rng.Next(5), default);
            orchTask = Task.Run(() => orchestrator.RunOnceAsync(default));
        }

        await Task.WhenAll(orchTask, bootTask);

        await Assert.That(queue.PublishCalls).IsLessThanOrEqualTo(1)
            .Because($"iteration {seed}: at most one publish expected per contention round");
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
