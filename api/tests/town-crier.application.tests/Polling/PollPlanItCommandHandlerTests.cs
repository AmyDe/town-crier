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
        var timeProvider = TimeProvider.System;
        var handler = new PollPlanItCommandHandler(planItClient, pollStateStore, timeProvider);

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
        var timeProvider = TimeProvider.System;
        var handler = new PollPlanItCommandHandler(planItClient, pollStateStore, timeProvider);

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
        var timeProvider = TimeProvider.System;
        var handler = new PollPlanItCommandHandler(planItClient, pollStateStore, timeProvider);

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
        var timeProvider = TimeProvider.System;
        var handler = new PollPlanItCommandHandler(planItClient, pollStateStore, timeProvider);

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
        var fakeTime = new DateTimeOffset(2026, 3, 16, 12, 0, 0, TimeSpan.Zero);
        var fakeTimeProvider = new FakeTimeProvider(fakeTime);
        var handler = new PollPlanItCommandHandler(planItClient, pollStateStore, fakeTimeProvider);

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(pollStateStore.LastPollTime).IsEqualTo(fakeTime);
    }
}
