using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.UserProfiles;

public sealed class RecordUserActivityCommandHandlerTests
{
    [Test]
    public async Task Should_UpdateLastActiveAt_When_ProfileExistsAndLastActiveIsStale()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var staleTime = new DateTimeOffset(2026, 3, 1, 0, 0, 0, TimeSpan.Zero);
        var profile = UserProfile.Register("auth0|user-1", email: null, now: staleTime);
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new RecordUserActivityCommandHandler(repository);
        var now = new DateTimeOffset(2026, 4, 20, 12, 0, 0, TimeSpan.Zero);

        // Act
        await handler.HandleAsync(new RecordUserActivityCommand("auth0|user-1", now), CancellationToken.None);

        // Assert
        var saved = repository.GetByUserId("auth0|user-1");
        await Assert.That(saved!.LastActiveAt).IsEqualTo(now);
    }

    [Test]
    public async Task Should_DoNothing_When_ProfileDoesNotExist()
    {
        // Arrange — unknown userId shouldn't throw or create a profile.
        var repository = new FakeUserProfileRepository();
        var handler = new RecordUserActivityCommandHandler(repository);

        // Act
        await handler.HandleAsync(
            new RecordUserActivityCommand("auth0|unknown", DateTimeOffset.UtcNow),
            CancellationToken.None);

        // Assert
        await Assert.That(repository.Count).IsEqualTo(0);
    }

    [Test]
    public async Task Should_SkipWrite_When_LastActivityRecordedWithinTwentyFourHours()
    {
        // Arrange — profile was active 1 hour ago; another request within the day
        // should be a no-op write to avoid hammering Cosmos on every request.
        var recentTime = new DateTimeOffset(2026, 4, 20, 11, 0, 0, TimeSpan.Zero);
        var repository = new SaveCountingUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-1", email: null, now: recentTime);
        await repository.SaveAsync(profile, CancellationToken.None);
        repository.ResetSaveCount();

        var handler = new RecordUserActivityCommandHandler(repository);
        var now = new DateTimeOffset(2026, 4, 20, 12, 0, 0, TimeSpan.Zero);

        // Act
        await handler.HandleAsync(new RecordUserActivityCommand("auth0|user-1", now), CancellationToken.None);

        // Assert — no write
        await Assert.That(repository.SaveCount).IsEqualTo(0);

        // LastActiveAt remains the earlier value.
        var saved = await repository.GetByUserIdAsync("auth0|user-1", CancellationToken.None);
        await Assert.That(saved!.LastActiveAt).IsEqualTo(recentTime);
    }

    [Test]
    public async Task Should_WriteOnce_When_LastActivityOlderThanTwentyFourHours()
    {
        // Arrange — LastActiveAt is 25 hours ago.
        var staleTime = new DateTimeOffset(2026, 4, 19, 10, 0, 0, TimeSpan.Zero);
        var repository = new SaveCountingUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-1", email: null, now: staleTime);
        await repository.SaveAsync(profile, CancellationToken.None);
        repository.ResetSaveCount();

        var handler = new RecordUserActivityCommandHandler(repository);
        var now = new DateTimeOffset(2026, 4, 20, 12, 0, 0, TimeSpan.Zero);

        // Act
        await handler.HandleAsync(new RecordUserActivityCommand("auth0|user-1", now), CancellationToken.None);

        // Assert
        await Assert.That(repository.SaveCount).IsEqualTo(1);
        var saved = await repository.GetByUserIdAsync("auth0|user-1", CancellationToken.None);
        await Assert.That(saved!.LastActiveAt).IsEqualTo(now);
    }
}
