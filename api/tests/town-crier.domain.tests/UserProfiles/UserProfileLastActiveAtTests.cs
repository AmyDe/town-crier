using TownCrier.Domain.UserProfiles;

namespace TownCrier.Domain.Tests.UserProfiles;

public sealed class UserProfileLastActiveAtTests
{
    [Test]
    public async Task Should_RecordNow_When_NewProfileRegistered()
    {
        // Arrange
        var now = new DateTimeOffset(2026, 4, 20, 12, 0, 0, TimeSpan.Zero);

        // Act
        var profile = UserProfile.Register("auth0|user-1", email: null, now: now);

        // Assert — new profiles are active as of registration time.
        await Assert.That(profile.LastActiveAt).IsEqualTo(now);
    }

    [Test]
    public async Task Should_UpdateLastActiveAt_When_RecordActivityCalled()
    {
        // Arrange
        var registeredAt = new DateTimeOffset(2026, 4, 1, 0, 0, 0, TimeSpan.Zero);
        var visitedAt = new DateTimeOffset(2026, 4, 20, 9, 30, 0, TimeSpan.Zero);
        var profile = UserProfile.Register("auth0|user-1", email: null, now: registeredAt);

        // Act
        profile.RecordActivity(visitedAt);

        // Assert
        await Assert.That(profile.LastActiveAt).IsEqualTo(visitedAt);
    }

    [Test]
    public async Task Should_NotRewindLastActiveAt_When_OlderActivityRecorded()
    {
        // Arrange
        var registeredAt = new DateTimeOffset(2026, 4, 1, 0, 0, 0, TimeSpan.Zero);
        var olderVisit = new DateTimeOffset(2026, 3, 31, 23, 0, 0, TimeSpan.Zero);
        var profile = UserProfile.Register("auth0|user-1", email: null, now: registeredAt);

        // Act
        profile.RecordActivity(olderVisit);

        // Assert — never rewind the clock backwards.
        await Assert.That(profile.LastActiveAt).IsEqualTo(registeredAt);
    }
}
