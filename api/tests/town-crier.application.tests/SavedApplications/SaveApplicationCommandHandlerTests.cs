using TownCrier.Application.SavedApplications;

namespace TownCrier.Application.Tests.SavedApplications;

public sealed class SaveApplicationCommandHandlerTests
{
    [Test]
    public async Task Should_SaveApplication_When_NotAlreadySaved()
    {
        // Arrange
        var repository = new FakeSavedApplicationRepository();
        var timeProvider = new FakeTimeProvider(new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero));
        var handler = new SaveApplicationCommandHandler(repository, timeProvider);
        var command = new SaveApplicationCommand("auth0|user-1", "planit-uid-abc");

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var saved = await repository.GetByUserIdAsync("auth0|user-1", CancellationToken.None);
        await Assert.That(saved).HasCount().EqualTo(1);
        await Assert.That(saved[0].ApplicationUid).IsEqualTo("planit-uid-abc");
        await Assert.That(saved[0].SavedAt).IsEqualTo(new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero));
    }

    [Test]
    public async Task Should_BeIdempotent_When_ApplicationAlreadySaved()
    {
        // Arrange
        var repository = new FakeSavedApplicationRepository();
        var timeProvider = new FakeTimeProvider(new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero));
        var handler = new SaveApplicationCommandHandler(repository, timeProvider);
        var command = new SaveApplicationCommand("auth0|user-1", "planit-uid-abc");

        await handler.HandleAsync(command, CancellationToken.None);

        // Act — save again
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert — still only one entry
        var saved = await repository.GetByUserIdAsync("auth0|user-1", CancellationToken.None);
        await Assert.That(saved).HasCount().EqualTo(1);
    }
}
