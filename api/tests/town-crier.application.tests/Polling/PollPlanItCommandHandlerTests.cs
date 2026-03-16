using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

public sealed class PollPlanItCommandHandlerTests
{
    [Test]
    public async Task Should_ReturnApplicationCount_When_PlanItReturnsApplications()
    {
        // Arrange
        var planItClient = new FakePlanItClient();
        planItClient.Add(new PlanningApplicationBuilder().WithUid("app-1").Build());
        planItClient.Add(new PlanningApplicationBuilder().WithUid("app-2").Build());
        planItClient.Add(new PlanningApplicationBuilder().WithUid("app-3").Build());

        var pollStateStore = new FakePollStateStore();
        var repository = new FakePlanningApplicationRepository();
        var timeProvider = TimeProvider.System;
        var handler = new PollPlanItCommandHandler(planItClient, pollStateStore, repository, timeProvider);

        // Act
        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(result.ApplicationCount).IsEqualTo(3);
    }

    [Test]
    public async Task Should_ReturnZeroCount_When_PlanItReturnsNoApplications()
    {
        // Arrange
        var planItClient = new FakePlanItClient();
        var pollStateStore = new FakePollStateStore();
        var repository = new FakePlanningApplicationRepository();
        var timeProvider = TimeProvider.System;
        var handler = new PollPlanItCommandHandler(planItClient, pollStateStore, repository, timeProvider);

        // Act
        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(result.ApplicationCount).IsEqualTo(0);
    }

    [Test]
    public async Task Should_PassNullDifferentStart_When_NoPreviousPollState()
    {
        // Arrange
        var planItClient = new FakePlanItClient();
        var pollStateStore = new FakePollStateStore();
        var repository = new FakePlanningApplicationRepository();
        var timeProvider = TimeProvider.System;
        var handler = new PollPlanItCommandHandler(planItClient, pollStateStore, repository, timeProvider);

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(planItClient.LastDifferentStartUsed).IsNull();
    }

    [Test]
    public async Task Should_PassLastPollTime_When_PreviousPollStateExists()
    {
        // Arrange
        var planItClient = new FakePlanItClient();
        var pollStateStore = new FakePollStateStore();
        var lastPoll = new DateTimeOffset(2026, 3, 15, 10, 0, 0, TimeSpan.Zero);
        pollStateStore.SetLastPollTime(lastPoll);
        var repository = new FakePlanningApplicationRepository();
        var timeProvider = TimeProvider.System;
        var handler = new PollPlanItCommandHandler(planItClient, pollStateStore, repository, timeProvider);

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(planItClient.LastDifferentStartUsed).IsEqualTo(lastPoll);
    }

    [Test]
    public async Task Should_PersistCurrentTime_When_PollSucceeds()
    {
        // Arrange
        var planItClient = new FakePlanItClient();
        var pollStateStore = new FakePollStateStore();
        var repository = new FakePlanningApplicationRepository();
        var fakeTime = new DateTimeOffset(2026, 3, 16, 12, 0, 0, TimeSpan.Zero);
        var fakeTimeProvider = new FakeTimeProvider(fakeTime);
        var handler = new PollPlanItCommandHandler(planItClient, pollStateStore, repository, fakeTimeProvider);

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(pollStateStore.LastPollTime).IsEqualTo(fakeTime);
    }

    [Test]
    public async Task Should_UpsertAllApplications_When_PlanItReturnsResults()
    {
        // Arrange
        var planItClient = new FakePlanItClient();
        var app1 = new PlanningApplicationBuilder().WithUid("app-1").WithName("Council/app-1").Build();
        var app2 = new PlanningApplicationBuilder().WithUid("app-2").WithName("Council/app-2").Build();
        planItClient.Add(app1);
        planItClient.Add(app2);

        var pollStateStore = new FakePollStateStore();
        var repository = new FakePlanningApplicationRepository();
        var timeProvider = TimeProvider.System;
        var handler = new PollPlanItCommandHandler(planItClient, pollStateStore, repository, timeProvider);

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(repository.GetAll()).HasCount().EqualTo(2);
        await Assert.That(repository.GetByName("Council/app-1")).IsNotNull();
        await Assert.That(repository.GetByName("Council/app-2")).IsNotNull();
    }

    [Test]
    public async Task Should_UpsertIdempotently_When_SameApplicationPolledTwice()
    {
        // Arrange
        var planItClient = new FakePlanItClient();
        var app = new PlanningApplicationBuilder()
            .WithUid("app-1")
            .WithName("Council/app-1")
            .Build();
        planItClient.Add(app);

        var pollStateStore = new FakePollStateStore();
        var repository = new FakePlanningApplicationRepository();
        var timeProvider = TimeProvider.System;
        var handler = new PollPlanItCommandHandler(planItClient, pollStateStore, repository, timeProvider);

        // Act — poll twice with the same data
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — still only one application in the store
        await Assert.That(repository.GetAll()).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_UpdateExistingApplication_When_DataChanges()
    {
        // Arrange
        var planItClient = new FakePlanItClient();
        var original = new PlanningApplicationBuilder()
            .WithUid("app-1")
            .WithName("Council/app-1")
            .Build();
        planItClient.Add(original);

        var pollStateStore = new FakePollStateStore();
        var repository = new FakePlanningApplicationRepository();
        var timeProvider = TimeProvider.System;
        var handler = new PollPlanItCommandHandler(planItClient, pollStateStore, repository, timeProvider);

        // First poll
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Replace with updated version
        planItClient.Clear();
        var updated = new PlanningApplicationBuilder()
            .WithUid("app-1")
            .WithName("Council/app-1")
            .WithAppState("Decided")
            .Build();
        planItClient.Add(updated);

        // Act — second poll
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — repository has the updated version
        var stored = repository.GetByName("Council/app-1");
        await Assert.That(stored!.AppState).IsEqualTo("Decided");
    }

    [Test]
    public async Task Should_NotUpsertAnyApplications_When_PlanItReturnsEmpty()
    {
        // Arrange
        var planItClient = new FakePlanItClient();
        var pollStateStore = new FakePollStateStore();
        var repository = new FakePlanningApplicationRepository();
        var timeProvider = TimeProvider.System;
        var handler = new PollPlanItCommandHandler(planItClient, pollStateStore, repository, timeProvider);

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(repository.GetAll()).HasCount().EqualTo(0);
    }
}
