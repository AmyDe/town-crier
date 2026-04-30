using TownCrier.Domain.Notifications;

namespace TownCrier.Application.Tests.Notifications;

public sealed class FakeNotificationRepositoryEmailTests
{
    [Test]
    public async Task Should_ReturnOnlyUnsentEmails_When_GetUnsentEmailsByUserAsyncCalled()
    {
        // Arrange
        var repo = new FakeNotificationRepository();

        var unsent = Notification.Create(
            "user-1",
            "uid-0001",
            "APP/2026/0001",
            "zone-1",
            "1 High St",
            "Extension",
            "Householder",
            42,
            new DateTimeOffset(2026, 3, 15, 10, 0, 0, TimeSpan.Zero));

        var sent = Notification.Create(
            "user-1",
            "uid-0002",
            "APP/2026/0002",
            "zone-1",
            "2 High St",
            "Garage",
            "Householder",
            42,
            new DateTimeOffset(2026, 3, 15, 11, 0, 0, TimeSpan.Zero));
        sent.MarkEmailSent();

        repo.Seed(unsent);
        repo.Seed(sent);

        // Act
        var result = await repo.GetUnsentEmailsByUserAsync("user-1", CancellationToken.None);

        // Assert
        await Assert.That(result).HasCount().EqualTo(1);
        await Assert.That(result[0].ApplicationName).IsEqualTo("APP/2026/0001");
    }

    [Test]
    public async Task Should_ReturnOrderedByCreatedAt_When_GetUnsentEmailsByUserAsyncCalled()
    {
        // Arrange
        var repo = new FakeNotificationRepository();

        var older = Notification.Create(
            "user-1",
            "uid-0001",
            "APP/2026/0001",
            "zone-1",
            "1 High St",
            "Extension",
            "Householder",
            42,
            new DateTimeOffset(2026, 3, 15, 8, 0, 0, TimeSpan.Zero));

        var newer = Notification.Create(
            "user-1",
            "uid-0002",
            "APP/2026/0002",
            "zone-1",
            "2 High St",
            "Garage",
            "Householder",
            42,
            new DateTimeOffset(2026, 3, 15, 12, 0, 0, TimeSpan.Zero));

        repo.Seed(newer);
        repo.Seed(older);

        // Act
        var result = await repo.GetUnsentEmailsByUserAsync("user-1", CancellationToken.None);

        // Assert
        await Assert.That(result).HasCount().EqualTo(2);
        await Assert.That(result[0].CreatedAt).IsLessThan(result[1].CreatedAt);
    }

    [Test]
    public async Task Should_ReturnEmpty_When_NoUnsentEmailsForUser()
    {
        // Arrange
        var repo = new FakeNotificationRepository();

        var sent = Notification.Create(
            "user-1",
            "uid-0001",
            "APP/2026/0001",
            "zone-1",
            "1 High St",
            "Extension",
            "Householder",
            42,
            new DateTimeOffset(2026, 3, 15, 10, 0, 0, TimeSpan.Zero));
        sent.MarkEmailSent();

        repo.Seed(sent);

        // Act
        var result = await repo.GetUnsentEmailsByUserAsync("user-1", CancellationToken.None);

        // Assert
        await Assert.That(result).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_NotReturnOtherUsersNotifications_When_GetUnsentEmailsByUserAsyncCalled()
    {
        // Arrange
        var repo = new FakeNotificationRepository();

        var user1Notification = Notification.Create(
            "user-1",
            "uid-0001",
            "APP/2026/0001",
            "zone-1",
            "1 High St",
            "Extension",
            "Householder",
            42,
            new DateTimeOffset(2026, 3, 15, 10, 0, 0, TimeSpan.Zero));

        var user2Notification = Notification.Create(
            "user-2",
            "uid-0002",
            "APP/2026/0002",
            "zone-2",
            "2 High St",
            "Garage",
            "Householder",
            42,
            new DateTimeOffset(2026, 3, 15, 11, 0, 0, TimeSpan.Zero));

        repo.Seed(user1Notification);
        repo.Seed(user2Notification);

        // Act
        var result = await repo.GetUnsentEmailsByUserAsync("user-1", CancellationToken.None);

        // Assert
        await Assert.That(result).HasCount().EqualTo(1);
        await Assert.That(result[0].UserId).IsEqualTo("user-1");
    }
}
