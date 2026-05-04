using System.Net;
using System.Net.Http.Headers;
using Microsoft.Extensions.DependencyInjection;
using TownCrier.Application.Notifications;
using TownCrier.Application.NotificationState;
using TownCrier.Domain.Notifications;
using TownCrier.Domain.NotificationState;
using TownCrier.Web.Tests.Auth;

namespace TownCrier.Web.Tests.NotificationState;

public sealed class NotificationStateEndpointTests
{
    [Test]
    public async Task Should_Return401_When_GetNotificationStateCalledWithoutToken()
    {
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        using var response = await client.GetAsync(
            new Uri("/v1/me/notification-state", UriKind.Relative));

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.Unauthorized);
    }

    [Test]
    public async Task Should_ReturnSeededWatermarkAndZeroUnread_When_FirstTouchUserCallsGet()
    {
        // Arrange — first-touch user has no document yet. The endpoint must
        // seed a fresh watermark at "now" and return zero unread because no
        // notification can be strictly newer than the seed instant.
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();
        client.DefaultRequestHeaders.Authorization =
            new AuthenticationHeaderValue("Bearer", TestJwtToken.Generate());

        // Act
        using var response = await client.GetAsync(
            new Uri("/v1/me/notification-state", UriKind.Relative));

        // Assert
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);
        var body = await response.Content.ReadAsStringAsync();
        await Assert.That(body).Contains("\"lastReadAt\":");
        await Assert.That(body).Contains("\"version\":1");
        await Assert.That(body).Contains("\"totalUnreadCount\":0");
    }

    [Test]
    public async Task Should_ReturnUnreadCount_When_NotificationsExistAfterWatermark()
    {
        // Arrange — seed a watermark in the past, then add a notification that
        // is strictly newer than it. The unread count must reflect that one row.
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();
        client.DefaultRequestHeaders.Authorization =
            new AuthenticationHeaderValue("Bearer", TestJwtToken.Generate());

        var userId = "auth0|test-user-123";
        using (var scope = factory.Services.CreateScope())
        {
            var stateRepo = scope.ServiceProvider
                .GetRequiredService<INotificationStateRepository>();
            var notifRepo = scope.ServiceProvider
                .GetRequiredService<INotificationRepository>();

            var state = NotificationStateAggregate.Create(
                userId, new DateTimeOffset(2026, 1, 1, 0, 0, 0, TimeSpan.Zero));
            await stateRepo.SaveAsync(state, CancellationToken.None);

            var notification = Notification.Create(
                userId: userId,
                applicationUid: "test-app-1",
                applicationName: "Test app",
                watchZoneId: null,
                applicationAddress: "1 Test Lane",
                applicationDescription: "Test description",
                applicationType: null,
                authorityId: 1,
                now: new DateTimeOffset(2026, 2, 1, 0, 0, 0, TimeSpan.Zero));
            await notifRepo.SaveAsync(notification, CancellationToken.None);
        }

        // Act
        using var response = await client.GetAsync(
            new Uri("/v1/me/notification-state", UriKind.Relative));

        // Assert
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);
        var body = await response.Content.ReadAsStringAsync();
        await Assert.That(body).Contains("\"totalUnreadCount\":1");
    }

    [Test]
    public async Task Should_Return401_When_MarkAllReadCalledWithoutToken()
    {
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        using var response = await client.PostAsync(
            new Uri("/v1/me/notification-state/mark-all-read", UriKind.Relative),
            content: null);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.Unauthorized);
    }

    [Test]
    public async Task Should_Return204AndStampWatermark_When_MarkAllReadCalled()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();
        client.DefaultRequestHeaders.Authorization =
            new AuthenticationHeaderValue("Bearer", TestJwtToken.Generate());

        // Act
        using var response = await client.PostAsync(
            new Uri("/v1/me/notification-state/mark-all-read", UriKind.Relative),
            content: null);

        // Assert
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.NoContent);

        using var scope = factory.Services.CreateScope();
        var stateRepo = scope.ServiceProvider
            .GetRequiredService<INotificationStateRepository>();
        var saved = await stateRepo.GetByUserIdAsync(
            "auth0|test-user-123", CancellationToken.None);
        await Assert.That(saved).IsNotNull();
    }

    [Test]
    public async Task Should_Return401_When_AdvanceCalledWithoutToken()
    {
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        using var content = new StringContent(
            """{"asOf":"2026-05-01T12:00:00Z"}""",
            System.Text.Encoding.UTF8,
            "application/json");
        using var response = await client.PostAsync(
            new Uri("/v1/me/notification-state/advance", UriKind.Relative),
            content);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.Unauthorized);
    }

    [Test]
    public async Task Should_Return204AndAdvanceWatermark_When_AdvanceCalledWithFutureInstant()
    {
        // Arrange — seed a watermark in the past, then advance it forward.
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();
        client.DefaultRequestHeaders.Authorization =
            new AuthenticationHeaderValue("Bearer", TestJwtToken.Generate());

        var userId = "auth0|test-user-123";
        var seedInstant = new DateTimeOffset(2026, 1, 1, 0, 0, 0, TimeSpan.Zero);
        var advanceTo = new DateTimeOffset(2026, 3, 15, 12, 0, 0, TimeSpan.Zero);

        using (var scope = factory.Services.CreateScope())
        {
            var stateRepo = scope.ServiceProvider
                .GetRequiredService<INotificationStateRepository>();
            var seed = NotificationStateAggregate.Create(userId, seedInstant);
            await stateRepo.SaveAsync(seed, CancellationToken.None);
        }

        // Act
        using var content = new StringContent(
            $$"""{"asOf":"{{advanceTo:O}}"}""",
            System.Text.Encoding.UTF8,
            "application/json");
        using var response = await client.PostAsync(
            new Uri("/v1/me/notification-state/advance", UriKind.Relative),
            content);

        // Assert
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.NoContent);

        using var verifyScope = factory.Services.CreateScope();
        var verifyRepo = verifyScope.ServiceProvider
            .GetRequiredService<INotificationStateRepository>();
        var saved = await verifyRepo.GetByUserIdAsync(userId, CancellationToken.None);
        await Assert.That(saved).IsNotNull();
        await Assert.That(saved!.LastReadAt).IsEqualTo(advanceTo);
    }

    [Test]
    public async Task Should_Return204AndKeepWatermark_When_AdvanceCalledWithStaleInstant()
    {
        // Arrange — seed a watermark, then attempt to advance to an instant
        // before it. The aggregate is monotonic; the watermark must not move.
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();
        client.DefaultRequestHeaders.Authorization =
            new AuthenticationHeaderValue("Bearer", TestJwtToken.Generate());

        var userId = "auth0|test-user-123";
        var seedInstant = new DateTimeOffset(2026, 5, 1, 0, 0, 0, TimeSpan.Zero);
        var staleInstant = new DateTimeOffset(2026, 1, 1, 0, 0, 0, TimeSpan.Zero);

        using (var scope = factory.Services.CreateScope())
        {
            var stateRepo = scope.ServiceProvider
                .GetRequiredService<INotificationStateRepository>();
            var seed = NotificationStateAggregate.Create(userId, seedInstant);
            await stateRepo.SaveAsync(seed, CancellationToken.None);
        }

        // Act
        using var content = new StringContent(
            $$"""{"asOf":"{{staleInstant:O}}"}""",
            System.Text.Encoding.UTF8,
            "application/json");
        using var response = await client.PostAsync(
            new Uri("/v1/me/notification-state/advance", UriKind.Relative),
            content);

        // Assert
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.NoContent);

        using var verifyScope = factory.Services.CreateScope();
        var verifyRepo = verifyScope.ServiceProvider
            .GetRequiredService<INotificationStateRepository>();
        var saved = await verifyRepo.GetByUserIdAsync(userId, CancellationToken.None);
        await Assert.That(saved).IsNotNull();
        await Assert.That(saved!.LastReadAt).IsEqualTo(seedInstant);
    }
}
