using TownCrier.Application.SavedApplications;
using TownCrier.Domain.SavedApplications;

namespace TownCrier.Application.Tests.SavedApplications;

public sealed class RemoveSavedApplicationCommandHandlerTests
{
    [Test]
    public async Task Should_RemoveApplication_When_PreviouslySaved()
    {
        // Arrange
        var repository = new FakeSavedApplicationRepository();
        var saved = SavedApplication.Create("auth0|user-1", "planit-uid-abc", DateTimeOffset.UtcNow);
        await repository.SaveAsync(saved, CancellationToken.None);

        var handler = new RemoveSavedApplicationCommandHandler(repository);
        var command = new RemoveSavedApplicationCommand("auth0|user-1", "planit-uid-abc");

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var remaining = await repository.GetByUserIdAsync("auth0|user-1", CancellationToken.None);
        await Assert.That(remaining).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_BeIdempotent_When_ApplicationNotSaved()
    {
        // Arrange
        var repository = new FakeSavedApplicationRepository();
        var handler = new RemoveSavedApplicationCommandHandler(repository);
        var command = new RemoveSavedApplicationCommand("auth0|user-1", "planit-uid-abc");

        // Act & Assert — should not throw
        await handler.HandleAsync(command, CancellationToken.None);
    }

    [Test]
    public async Task Should_OnlyRemoveTargetedApplication_When_UserHasMultipleSaved()
    {
        // Arrange
        var repository = new FakeSavedApplicationRepository();
        await repository.SaveAsync(
            SavedApplication.Create("auth0|user-1", "planit-uid-abc", DateTimeOffset.UtcNow), CancellationToken.None);
        await repository.SaveAsync(
            SavedApplication.Create("auth0|user-1", "planit-uid-def", DateTimeOffset.UtcNow), CancellationToken.None);

        var handler = new RemoveSavedApplicationCommandHandler(repository);
        var command = new RemoveSavedApplicationCommand("auth0|user-1", "planit-uid-abc");

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var remaining = await repository.GetByUserIdAsync("auth0|user-1", CancellationToken.None);
        await Assert.That(remaining).HasCount().EqualTo(1);
        await Assert.That(remaining[0].ApplicationUid).IsEqualTo("planit-uid-def");
    }
}
