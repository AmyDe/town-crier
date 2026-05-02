using TownCrier.Application.SavedApplications;
using TownCrier.Application.Tests.Polling;

namespace TownCrier.Application.Tests.SavedApplications;

public sealed class SaveApplicationCommandHandlerTests
{
    [Test]
    public async Task Should_UpsertApplication_AndSave_When_NotAlreadySaved()
    {
        // Arrange. Save flow now carries full PlanningApplication so the
        // application is persisted to Cosmos at save time, instead of being
        // upserted by the search hot loop. See bead tc-if12.
        var savedRepository = new FakeSavedApplicationRepository();
        var planningRepository = new FakePlanningApplicationRepository();
        var timeProvider = new FakeTimeProvider(new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero));
        var handler = new SaveApplicationCommandHandler(savedRepository, planningRepository, timeProvider);

        var application = new PlanningApplicationBuilder()
            .WithUid("planit-uid-abc")
            .WithName("Camden/CAM/24/0042/FUL")
            .WithAreaId(42)
            .WithAreaName("Camden")
            .Build();
        var command = new SaveApplicationCommand("auth0|user-1", application);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert — saved record persisted.
        var saved = await savedRepository.GetByUserIdAsync("auth0|user-1", CancellationToken.None);
        await Assert.That(saved).HasCount().EqualTo(1);
        await Assert.That(saved[0].ApplicationUid).IsEqualTo("planit-uid-abc");
        await Assert.That(saved[0].SavedAt).IsEqualTo(new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero));

        // Assert — application was also upserted to the planning repository.
        await Assert.That(planningRepository.UpsertCallCount).IsEqualTo(1);
        var stored = await planningRepository.GetByUidAsync("planit-uid-abc", CancellationToken.None);
        await Assert.That(stored).IsNotNull();
        await Assert.That(stored!.Name).IsEqualTo("Camden/CAM/24/0042/FUL");
    }

    [Test]
    public async Task Should_BeIdempotent_When_ApplicationAlreadySaved()
    {
        // Arrange
        var savedRepository = new FakeSavedApplicationRepository();
        var planningRepository = new FakePlanningApplicationRepository();
        var timeProvider = new FakeTimeProvider(new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero));
        var handler = new SaveApplicationCommandHandler(savedRepository, planningRepository, timeProvider);

        var application = new PlanningApplicationBuilder()
            .WithUid("planit-uid-abc")
            .Build();
        var command = new SaveApplicationCommand("auth0|user-1", application);

        await handler.HandleAsync(command, CancellationToken.None);

        // Act — save again
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert — still only one entry
        var saved = await savedRepository.GetByUserIdAsync("auth0|user-1", CancellationToken.None);
        await Assert.That(saved).HasCount().EqualTo(1);
    }
}
