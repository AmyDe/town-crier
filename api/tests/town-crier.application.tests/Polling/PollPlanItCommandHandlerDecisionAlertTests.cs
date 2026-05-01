using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

public sealed class PollPlanItCommandHandlerDecisionAlertTests
{
    [Test]
    public async Task Should_RaiseDecisionEvent_When_NewApplicationArrivesAlreadyDecided()
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

        var dispatcher = new FakeDecisionEventDispatcher();
        var handler = CreateHandler(
            planItClient: planItClient,
            authorityProvider: authorityProvider,
            decisionEventDispatcher: dispatcher);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(dispatcher.Dispatched).HasCount().EqualTo(1);
        await Assert.That(dispatcher.Dispatched[0].Uid).IsEqualTo("app-1");
    }

    [Test]
    [Arguments("Permitted")]
    [Arguments("Conditions")]
    [Arguments("Rejected")]
    [Arguments("Appealed")]
    public async Task Should_RaiseDecisionEvent_When_StateTransitionsFromUndecidedToDecision(string newState)
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var existing = new PlanningApplicationBuilder()
            .WithUid("app-1")
            .WithAreaId(1)
            .WithAppState("Undecided")
            .Build();
        var repository = new FakePlanningApplicationRepository();
        await repository.UpsertAsync(existing, CancellationToken.None);

        var planItClient = new FakePlanItClient();
        planItClient.Add(
            1,
            new PlanningApplicationBuilder()
                .WithUid("app-1")
                .WithAreaId(1)
                .WithAppState(newState)
                .Build());

        var dispatcher = new FakeDecisionEventDispatcher();
        var handler = CreateHandler(
            planItClient: planItClient,
            authorityProvider: authorityProvider,
            repository: repository,
            decisionEventDispatcher: dispatcher);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(dispatcher.Dispatched).HasCount().EqualTo(1);
        await Assert.That(dispatcher.Dispatched[0].AppState).IsEqualTo(newState);
    }

    [Test]
    [Arguments("Withdrawn")]
    [Arguments("Unresolved")]
    [Arguments("Referred")]
    public async Task Should_NotRaiseDecisionEvent_When_StateTransitionsFromUndecidedToNonDecision(string newState)
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var existing = new PlanningApplicationBuilder()
            .WithUid("app-1")
            .WithAreaId(1)
            .WithAppState("Undecided")
            .Build();
        var repository = new FakePlanningApplicationRepository();
        await repository.UpsertAsync(existing, CancellationToken.None);

        var planItClient = new FakePlanItClient();
        planItClient.Add(
            1,
            new PlanningApplicationBuilder()
                .WithUid("app-1")
                .WithAreaId(1)
                .WithAppState(newState)
                .Build());

        var dispatcher = new FakeDecisionEventDispatcher();
        var handler = CreateHandler(
            planItClient: planItClient,
            authorityProvider: authorityProvider,
            repository: repository,
            decisionEventDispatcher: dispatcher);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(dispatcher.Dispatched).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_NotRaiseDecisionEvent_When_DecidedStateChangesToAnotherDecidedState()
    {
        // Permitted -> Conditions is a same-decision-class change. The first
        // decision already triggered the event; downstream idempotency in
        // DispatchDecisionEventCommandHandler would suppress it anyway, but we
        // gate at the transition layer to keep telemetry honest.
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var existing = new PlanningApplicationBuilder()
            .WithUid("app-1")
            .WithAreaId(1)
            .WithAppState("Permitted")
            .Build();
        var repository = new FakePlanningApplicationRepository();
        await repository.UpsertAsync(existing, CancellationToken.None);

        var planItClient = new FakePlanItClient();
        planItClient.Add(
            1,
            new PlanningApplicationBuilder()
                .WithUid("app-1")
                .WithAreaId(1)
                .WithAppState("Conditions")
                .Build());

        var dispatcher = new FakeDecisionEventDispatcher();
        var handler = CreateHandler(
            planItClient: planItClient,
            authorityProvider: authorityProvider,
            repository: repository,
            decisionEventDispatcher: dispatcher);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(dispatcher.Dispatched).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_NotRaiseDecisionEvent_When_BusinessFieldsUnchanged()
    {
        // The hot-path optimisation in PollPlanItCommandHandler skips the upsert
        // and zone fan-out when HasSameBusinessFieldsAs returns true. The decision
        // event must also be skipped — re-dispatching on every rescrape of an
        // already-decided application would burn push budget.
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var existing = new PlanningApplicationBuilder()
            .WithUid("app-1")
            .WithAreaId(1)
            .WithAppState("Permitted")
            .Build();
        var repository = new FakePlanningApplicationRepository();
        await repository.UpsertAsync(existing, CancellationToken.None);

        // Same UID, area, name, address — same business fields. PlanIt simply
        // bumped LastDifferent on a rescrape.
        var planItClient = new FakePlanItClient();
        planItClient.Add(
            1,
            new PlanningApplicationBuilder()
                .WithUid("app-1")
                .WithAreaId(1)
                .WithAppState("Permitted")
                .Build());

        var dispatcher = new FakeDecisionEventDispatcher();
        var handler = CreateHandler(
            planItClient: planItClient,
            authorityProvider: authorityProvider,
            repository: repository,
            decisionEventDispatcher: dispatcher);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(dispatcher.Dispatched).HasCount().EqualTo(0);
    }

    private static PollPlanItCommandHandler CreateHandler(
        FakePlanItClient? planItClient = null,
        FakePollStateStore? pollStateStore = null,
        FakePlanningApplicationRepository? repository = null,
        FakeActiveAuthorityProvider? authorityProvider = null,
        FakeWatchZoneRepository? watchZoneRepository = null,
        FakeNotificationEnqueuer? notificationEnqueuer = null,
        FakeDecisionEventDispatcher? decisionEventDispatcher = null,
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
            decisionEventDispatcher ?? new FakeDecisionEventDispatcher(),
            cycleSelector ?? new FakeCycleSelector(CycleType.Watched),
            options ?? new PollingOptions(),
            NullLogger<PollPlanItCommandHandler>.Instance);
    }
}
