using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

public sealed class PollPlanItCommandHandlerLeaseTests
{
    [Test]
    public async Task Should_NotCallPlanIt_When_LeaseIsHeld()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var planItClient = new FakePlanItClient();
        planItClient.Add(1, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(1).Build());

        var lease = new FakePollingLeaseStore { AcquireResult = false };

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider, leaseStore: lease);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(planItClient.AuthorityIdsRequested).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_ReturnLeaseHeldTermination_When_LeaseIsHeld()
    {
        var lease = new FakePollingLeaseStore { AcquireResult = false };

        var handler = CreateHandler(leaseStore: lease);

        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(result.TerminationReason).IsEqualTo(PollTerminationReason.LeaseHeld);
        await Assert.That(result.ApplicationCount).IsEqualTo(0);
        await Assert.That(result.AuthoritiesPolled).IsEqualTo(0);
    }

    [Test]
    public async Task Should_ReleaseLease_After_CycleCompletesNaturally()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var planItClient = new FakePlanItClient();
        planItClient.Add(1, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(1).Build());

        var lease = new FakePollingLeaseStore { AcquireResult = true };

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider, leaseStore: lease);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(lease.AcquireCalls).IsEqualTo(1);
        await Assert.That(lease.ReleaseCalls).IsEqualTo(1);
    }

    [Test]
    public async Task Should_ReleaseLease_When_HandlerThrows()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var failingClient = new FakePlanItClient { ExceptionToThrow = new InvalidOperationException("boom") };

        var lease = new FakePollingLeaseStore { AcquireResult = true };

        var handler = CreateHandler(planItClient: failingClient, authorityProvider: authorityProvider, leaseStore: lease);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(lease.ReleaseCalls).IsEqualTo(1);
    }

    [Test]
    public async Task Should_NotReleaseLease_When_AcquireFails()
    {
        var lease = new FakePollingLeaseStore { AcquireResult = false };

        var handler = CreateHandler(leaseStore: lease);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(lease.AcquireCalls).IsEqualTo(1);
        await Assert.That(lease.ReleaseCalls).IsEqualTo(0);
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
}
