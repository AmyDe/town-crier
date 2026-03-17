using TownCrier.Application.Notifications;
using TownCrier.Domain.Notifications;

namespace TownCrier.Application.Tests.Notifications;

public sealed class GetNotificationsQueryHandlerTests
{
    private static readonly DateTimeOffset March2026 = new(2026, 3, 15, 10, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task Should_ReturnNotifications_When_UserHasNotifications()
    {
        // Arrange
        var (handler, repo) = CreateHandler();
        SeedNotifications(repo, "user-1", 3);

        // Act
        var result = await handler.HandleAsync(
            new GetNotificationsQuery("user-1", Page: 1, PageSize: 20), CancellationToken.None);

        // Assert
        await Assert.That(result.Notifications).HasCount().EqualTo(3);
        await Assert.That(result.Total).IsEqualTo(3);
        await Assert.That(result.Page).IsEqualTo(1);
    }

    [Test]
    public async Task Should_ReturnEmptyList_When_UserHasNoNotifications()
    {
        // Arrange
        var (handler, _) = CreateHandler();

        // Act
        var result = await handler.HandleAsync(
            new GetNotificationsQuery("user-1", Page: 1, PageSize: 20), CancellationToken.None);

        // Assert
        await Assert.That(result.Notifications).HasCount().EqualTo(0);
        await Assert.That(result.Total).IsEqualTo(0);
    }

    [Test]
    public async Task Should_ReturnMostRecentFirst_When_MultipleNotifications()
    {
        // Arrange
        var (handler, repo) = CreateHandler();

        var older = Notification.Create(
            "user-1",
            "app-001",
            "zone-1",
            "1 High St",
            "Extension",
            "Householder",
            1,
            March2026.AddDays(-2));
        var newer = Notification.Create(
            "user-1",
            "app-002",
            "zone-1",
            "2 High St",
            "New build",
            "Full",
            1,
            March2026);

        repo.Seed(older);
        repo.Seed(newer);

        // Act
        var result = await handler.HandleAsync(
            new GetNotificationsQuery("user-1", Page: 1, PageSize: 20), CancellationToken.None);

        // Assert
        await Assert.That(result.Notifications[0].ApplicationName).IsEqualTo("app-002");
        await Assert.That(result.Notifications[1].ApplicationName).IsEqualTo("app-001");
    }

    [Test]
    public async Task Should_OnlyReturnOwnNotifications_When_OtherUsersExist()
    {
        // Arrange
        var (handler, repo) = CreateHandler();
        SeedNotifications(repo, "user-1", 2);
        SeedNotifications(repo, "user-2", 3);

        // Act
        var result = await handler.HandleAsync(
            new GetNotificationsQuery("user-1", Page: 1, PageSize: 20), CancellationToken.None);

        // Assert
        await Assert.That(result.Notifications).HasCount().EqualTo(2);
        await Assert.That(result.Total).IsEqualTo(2);
    }

    [Test]
    public async Task Should_RespectPageSize_When_MoreNotificationsThanPageSize()
    {
        // Arrange
        var (handler, repo) = CreateHandler();
        SeedNotifications(repo, "user-1", 5);

        // Act
        var result = await handler.HandleAsync(
            new GetNotificationsQuery("user-1", Page: 1, PageSize: 2), CancellationToken.None);

        // Assert
        await Assert.That(result.Notifications).HasCount().EqualTo(2);
        await Assert.That(result.Total).IsEqualTo(5);
    }

    [Test]
    public async Task Should_ReturnSecondPage_When_PageIsTwo()
    {
        // Arrange
        var (handler, repo) = CreateHandler();

        for (var i = 0; i < 5; i++)
        {
            var notification = Notification.Create(
                "user-1",
                $"app-{i:D3}",
                "zone-1",
                "Address",
                "Desc",
                "Type",
                1,
                March2026.AddHours(-i));
            repo.Seed(notification);
        }

        // Act
        var result = await handler.HandleAsync(
            new GetNotificationsQuery("user-1", Page: 2, PageSize: 2), CancellationToken.None);

        // Assert
        await Assert.That(result.Notifications).HasCount().EqualTo(2);
        await Assert.That(result.Notifications[0].ApplicationName).IsEqualTo("app-002");
        await Assert.That(result.Notifications[1].ApplicationName).IsEqualTo("app-003");
    }

    [Test]
    public async Task Should_IncludeNotificationDetails_When_Returned()
    {
        // Arrange
        var (handler, repo) = CreateHandler();

        var notification = Notification.Create(
            "user-1",
            "app-001",
            "zone-1",
            "1 High St",
            "Rear extension",
            "Householder",
            42,
            March2026);
        repo.Seed(notification);

        // Act
        var result = await handler.HandleAsync(
            new GetNotificationsQuery("user-1", Page: 1, PageSize: 20), CancellationToken.None);

        // Assert
        var item = result.Notifications[0];
        await Assert.That(item.ApplicationName).IsEqualTo("app-001");
        await Assert.That(item.ApplicationAddress).IsEqualTo("1 High St");
        await Assert.That(item.ApplicationDescription).IsEqualTo("Rear extension");
        await Assert.That(item.ApplicationType).IsEqualTo("Householder");
        await Assert.That(item.AuthorityId).IsEqualTo(42);
        await Assert.That(item.CreatedAt).IsEqualTo(March2026);
    }

    private static (GetNotificationsQueryHandler Handler, FakeNotificationRepository Repo) CreateHandler()
    {
        var repo = new FakeNotificationRepository();
        var handler = new GetNotificationsQueryHandler(repo);
        return (handler, repo);
    }

    private static void SeedNotifications(FakeNotificationRepository repo, string userId, int count)
    {
        for (var i = 0; i < count; i++)
        {
            var notification = Notification.Create(
                userId,
                $"{userId}-app-{i:D3}",
                "zone-1",
                "Address",
                "Description",
                "Type",
                1,
                March2026.AddHours(-i));
            repo.Seed(notification);
        }
    }
}
