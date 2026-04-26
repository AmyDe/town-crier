using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

public sealed class PollPlanItCommandHandlerDecisionAlertTests
{
    [Test]
    public async Task Should_DispatchDecisionAlert_When_NewApplicationArrivesAlreadyDecided()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var planItClient = new FakePlanItClient();
        planItClient.Add(
            1,
            new PlanningApplicationBuilder()
                .WithUid("app-1")
                .WithAreaId(1)
                .WithAppState("Permitted")
                .Build());

        var dispatcher = new FakeDecisionAlertDispatcher();
        var handler = CreateHandler(
            planItClient: planItClient,
            authorityProvider: authorityProvider,
            decisionAlertDispatcher: dispatcher);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(dispatcher.Dispatched).HasCount().EqualTo(1);
        await Assert.That(dispatcher.Dispatched[0].Uid).IsEqualTo("app-1");
    }

    private static PollPlanItCommandHandler CreateHandler(
        FakePlanItClient? planItClient = null,
        FakePollStateStore? pollStateStore = null,
        FakePlanningApplicationRepository? repository = null,
        FakeActiveAuthorityProvider? authorityProvider = null,
        FakeWatchZoneRepository? watchZoneRepository = null,
        FakeNotificationEnqueuer? notificationEnqueuer = null,
        FakeDecisionAlertDispatcher? decisionAlertDispatcher = null,
        TimeProvider? timeProvider = null,
        ICycleSelector? cycleSelector = null,
        PollingOptions? options = null)
    {
        return new PollPlanItCommandHandler(
            planItClient ?? new FakePlanItClient(),
            pollStateStore ?? new FakePollStateStore(),
            repository ?? new FakePlanningApplicationRepository(),
            timeProvider ?? TimeProvider.System,
            authorityProvider ?? new FakeActiveAuthorityProvider(),
            watchZoneRepository ?? new FakeWatchZoneRepository(),
            notificationEnqueuer ?? new FakeNotificationEnqueuer(),
            decisionAlertDispatcher ?? new FakeDecisionAlertDispatcher(),
            cycleSelector ?? new FakeCycleSelector(CycleType.Watched),
            options ?? new PollingOptions(),
            NullLogger<PollPlanItCommandHandler>.Instance);
    }
}
