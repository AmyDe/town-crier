using TownCrier.Domain.Notifications;

namespace TownCrier.Application.Tests.Notifications;

public sealed class FakeNotificationRepositoryUserIdsTests
{
    [Test]
    public async Task Should_ReturnDistinctUserIds_When_MultipleUsersHaveUnsentEmails()
    {
        // Arrange
        var repo = new FakeNotificationRepository();

        var n1 = Notification.Create(
            "user-1",
            "uid-001",
            "APP/001",
            "zone-1",
            "1 High St",
            "Extension",
            "Householder",
            42,
            new DateTimeOffset(2026, 3, 15, 10, 0, 0, TimeSpan.Zero));
        var n2 = Notification.Create(
            "user-2",
            "uid-002",
            "APP/002",
            "zone-2",
            "2 High St",
            "Garage",
            "Householder",
            42,
            new DateTimeOffset(2026, 3, 15, 11, 0, 0, TimeSpan.Zero));
        var n3 = Notification.Create(
            "user-1",
            "uid-003",
            "APP/003",
            "zone-1",
            "3 High St",
            "Loft",
            "Householder",
            42,
            new DateTimeOffset(2026, 3, 15, 12, 0, 0, TimeSpan.Zero));

        repo.Seed(n1);
        repo.Seed(n2);
        repo.Seed(n3);

        // Act
        var result = await repo.GetUserIdsWithUnsentEmailsAsync(CancellationToken.None);

        // Assert
        await Assert.That(result).HasCount().EqualTo(2);
        await Assert.That(result).Contains("user-1");
        await Assert.That(result).Contains("user-2");
    }

    [Test]
    public async Task Should_ExcludeUsersWithAllEmailsSent_When_GetUserIdsWithUnsentEmailsAsyncCalled()
    {
        // Arrange
        var repo = new FakeNotificationRepository();

        var unsent = Notification.Create(
            "user-1",
            "uid-001",
            "APP/001",
            "zone-1",
            "1 High St",
            "Extension",
            "Householder",
            42,
            new DateTimeOffset(2026, 3, 15, 10, 0, 0, TimeSpan.Zero));

        var sent = Notification.Create(
            "user-2",
            "uid-002",
            "APP/002",
            "zone-2",
            "2 High St",
            "Garage",
            "Householder",
            42,
            new DateTimeOffset(2026, 3, 15, 11, 0, 0, TimeSpan.Zero));
        sent.MarkEmailSent();

        repo.Seed(unsent);
        repo.Seed(sent);

        // Act
        var result = await repo.GetUserIdsWithUnsentEmailsAsync(CancellationToken.None);

        // Assert
        await Assert.That(result).HasCount().EqualTo(1);
        await Assert.That(result).Contains("user-1");
    }

    [Test]
    public async Task Should_ReturnEmpty_When_NoUnsentEmails()
    {
        // Arrange
        var repo = new FakeNotificationRepository();

        var sent = Notification.Create(
            "user-1",
            "uid-001",
            "APP/001",
            "zone-1",
            "1 High St",
            "Extension",
            "Householder",
            42,
            new DateTimeOffset(2026, 3, 15, 10, 0, 0, TimeSpan.Zero));
        sent.MarkEmailSent();

        repo.Seed(sent);

        // Act
        var result = await repo.GetUserIdsWithUnsentEmailsAsync(CancellationToken.None);

        // Assert
        await Assert.That(result).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_CountOnlyNotificationsAfterWatermark_When_GetUnreadCountAsyncCalled()
    {
        // Arrange — three notifications across the watermark boundary. Two created
        // strictly after lastReadAt are unread; the one created at the watermark
        // is read (boundary is exclusive — `createdAt > lastReadAt`).
        var repo = new FakeNotificationRepository();
        var lastReadAt = new DateTimeOffset(2026, 3, 15, 12, 0, 0, TimeSpan.Zero);

        var beforeWatermark = Notification.Create(
            "user-1",
            "uid-001",
            "APP/001",
            "zone-1",
            "1 High St",
            "Extension",
            "Householder",
            42,
            new DateTimeOffset(2026, 3, 15, 10, 0, 0, TimeSpan.Zero));
        var atWatermark = Notification.Create(
            "user-1",
            "uid-002",
            "APP/002",
            "zone-1",
            "2 High St",
            "Garage",
            "Householder",
            42,
            lastReadAt);
        var afterWatermarkOne = Notification.Create(
            "user-1",
            "uid-003",
            "APP/003",
            "zone-1",
            "3 High St",
            "Loft",
            "Householder",
            42,
            new DateTimeOffset(2026, 3, 15, 13, 0, 0, TimeSpan.Zero));
        var afterWatermarkTwo = Notification.Create(
            "user-1",
            "uid-004",
            "APP/004",
            "zone-1",
            "4 High St",
            "Loft",
            "Householder",
            42,
            new DateTimeOffset(2026, 3, 15, 14, 0, 0, TimeSpan.Zero));

        repo.Seed(beforeWatermark);
        repo.Seed(atWatermark);
        repo.Seed(afterWatermarkOne);
        repo.Seed(afterWatermarkTwo);

        // Act
        var result = await repo.GetUnreadCountAsync("user-1", lastReadAt, CancellationToken.None);

        // Assert — only the two created strictly after lastReadAt
        await Assert.That(result).IsEqualTo(2);
    }

    [Test]
    public async Task Should_ScopeUnreadCountToUser_When_OtherUsersHaveNotifications()
    {
        // Arrange — two users with overlapping post-watermark notifications.
        // GetUnreadCountAsync must only count the requested user's rows.
        var repo = new FakeNotificationRepository();
        var lastReadAt = new DateTimeOffset(2026, 3, 15, 10, 0, 0, TimeSpan.Zero);

        var userOne = Notification.Create(
            "user-1",
            "uid-001",
            "APP/001",
            "zone-1",
            "1 High St",
            "Extension",
            "Householder",
            42,
            new DateTimeOffset(2026, 3, 15, 11, 0, 0, TimeSpan.Zero));
        var userTwoOne = Notification.Create(
            "user-2",
            "uid-002",
            "APP/002",
            "zone-2",
            "2 High St",
            "Garage",
            "Householder",
            42,
            new DateTimeOffset(2026, 3, 15, 11, 30, 0, TimeSpan.Zero));
        var userTwoTwo = Notification.Create(
            "user-2",
            "uid-003",
            "APP/003",
            "zone-2",
            "3 High St",
            "Loft",
            "Householder",
            42,
            new DateTimeOffset(2026, 3, 15, 12, 0, 0, TimeSpan.Zero));

        repo.Seed(userOne);
        repo.Seed(userTwoOne);
        repo.Seed(userTwoTwo);

        // Act
        var resultUserOne = await repo.GetUnreadCountAsync("user-1", lastReadAt, CancellationToken.None);
        var resultUserTwo = await repo.GetUnreadCountAsync("user-2", lastReadAt, CancellationToken.None);

        // Assert
        await Assert.That(resultUserOne).IsEqualTo(1);
        await Assert.That(resultUserTwo).IsEqualTo(2);
    }
}
