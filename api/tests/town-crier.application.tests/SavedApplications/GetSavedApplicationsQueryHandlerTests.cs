using TownCrier.Application.SavedApplications;
using TownCrier.Domain.SavedApplications;

namespace TownCrier.Application.Tests.SavedApplications;

public sealed class GetSavedApplicationsQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnEmptyList_When_UserHasNoSavedApplications()
    {
        // Arrange
        var repository = new FakeSavedApplicationRepository();
        var handler = new GetSavedApplicationsQueryHandler(repository);
        var query = new GetSavedApplicationsQuery("auth0|user-1");

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_ReturnSavedApplications_When_UserHasSaved()
    {
        // Arrange
        var repository = new FakeSavedApplicationRepository();
        var savedAt = new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero);
        await repository.SaveAsync(
            SavedApplication.Create("auth0|user-1", "planit-uid-abc", savedAt), CancellationToken.None);
        await repository.SaveAsync(
            SavedApplication.Create("auth0|user-1", "planit-uid-def", savedAt.AddHours(1)), CancellationToken.None);

        var handler = new GetSavedApplicationsQueryHandler(repository);
        var query = new GetSavedApplicationsQuery("auth0|user-1");

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result).HasCount().EqualTo(2);
        await Assert.That(result[0].ApplicationUid).IsEqualTo("planit-uid-abc");
        await Assert.That(result[1].ApplicationUid).IsEqualTo("planit-uid-def");
    }

    [Test]
    public async Task Should_OnlyReturnOwnApplications_When_MultipleUsersHaveSaved()
    {
        // Arrange
        var repository = new FakeSavedApplicationRepository();
        var savedAt = DateTimeOffset.UtcNow;
        await repository.SaveAsync(
            SavedApplication.Create("auth0|user-1", "planit-uid-abc", savedAt), CancellationToken.None);
        await repository.SaveAsync(
            SavedApplication.Create("auth0|user-2", "planit-uid-def", savedAt), CancellationToken.None);

        var handler = new GetSavedApplicationsQueryHandler(repository);
        var query = new GetSavedApplicationsQuery("auth0|user-1");

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result).HasCount().EqualTo(1);
        await Assert.That(result[0].ApplicationUid).IsEqualTo("planit-uid-abc");
    }
}
