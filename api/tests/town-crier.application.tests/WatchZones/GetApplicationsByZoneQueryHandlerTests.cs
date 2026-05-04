using TownCrier.Application.Tests.Notifications;
using TownCrier.Application.Tests.NotificationState;
using TownCrier.Application.Tests.Polling;
using TownCrier.Application.WatchZones;
using TownCrier.Domain.Notifications;
using TownCrier.Domain.NotificationState;

namespace TownCrier.Application.Tests.WatchZones;

public sealed class GetApplicationsByZoneQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnNearbyApplications_When_ZoneExists()
    {
        // Arrange — zone centred on Camden Town, 1 km radius
        var zone = new WatchZoneBuilder()
            .WithId("zone-1")
            .WithUserId("user-1")
            .WithName("My Zone")
            .WithCentre(51.5390, -0.1426)
            .WithRadiusMetres(1000)
            .WithAuthorityId(42)
            .Build();

        var watchZoneRepo = new FakeWatchZoneRepository();
        watchZoneRepo.Add(zone);

        // Application ~200m from centre (inside zone)
        var nearby = new PlanningApplicationBuilder()
            .WithName("nearby-app")
            .WithUid("uid-nearby")
            .WithAreaId(42)
            .WithCoordinates(51.5380, -0.1410)
            .Build();

        // Application ~5km from centre (outside zone)
        var far = new PlanningApplicationBuilder()
            .WithName("far-app")
            .WithUid("uid-far")
            .WithAreaId(42)
            .WithCoordinates(51.5074, -0.1278)
            .Build();

        var appRepo = new FakePlanningApplicationRepository();
        await appRepo.UpsertAsync(nearby, CancellationToken.None);
        await appRepo.UpsertAsync(far, CancellationToken.None);

        var handler = CreateHandler(watchZoneRepo, appRepo);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationsByZoneQuery("user-1", "zone-1"), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.Count).IsEqualTo(1);
        await Assert.That(result[0].Uid).IsEqualTo("uid-nearby");
    }

    [Test]
    public async Task Should_ReturnNull_When_ZoneNotFound()
    {
        // Arrange
        var watchZoneRepo = new FakeWatchZoneRepository();
        var appRepo = new FakePlanningApplicationRepository();
        var handler = CreateHandler(watchZoneRepo, appRepo);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationsByZoneQuery("user-1", "nonexistent"), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNull();
    }

    [Test]
    public async Task Should_ReturnNull_When_ZoneOwnedByDifferentUser()
    {
        // Arrange
        var zone = new WatchZoneBuilder()
            .WithId("zone-1")
            .WithUserId("other-user")
            .WithName("Their Zone")
            .WithCentre(51.5390, -0.1426)
            .WithRadiusMetres(1000)
            .WithAuthorityId(42)
            .Build();

        var watchZoneRepo = new FakeWatchZoneRepository();
        watchZoneRepo.Add(zone);

        var appRepo = new FakePlanningApplicationRepository();
        var handler = CreateHandler(watchZoneRepo, appRepo);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationsByZoneQuery("user-1", "zone-1"), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNull();
    }

    [Test]
    public async Task Should_PopulateLatestUnreadEvent_When_NotificationIsAfterWatermark()
    {
        // Arrange — user has a watermark of midday; one notification 1h after the
        // watermark exists for the application in scope, so it must surface as the
        // row's latestUnreadEvent.
        var watermark = new DateTimeOffset(2026, 5, 1, 12, 0, 0, TimeSpan.Zero);

        var (handler, notificationRepo, stateRepo) = CreateBuilt();
        SeedZone(stateRepo, watermark);

        var nearby = new PlanningApplicationBuilder()
            .WithName("nearby-app")
            .WithUid("uid-nearby")
            .WithAreaId(42)
            .WithCoordinates(51.5380, -0.1410)
            .Build();

        notificationRepo.Seed(BuildNotification(
            applicationUid: "uid-nearby",
            eventType: NotificationEventType.DecisionUpdate,
            decision: "Permitted",
            createdAt: watermark.AddHours(1)));

        var watchZoneRepo = new FakeWatchZoneRepository();
        watchZoneRepo.Add(BuildZone());

        var appRepo = new FakePlanningApplicationRepository();
        await appRepo.UpsertAsync(nearby, CancellationToken.None);

        // Rebuild handler now that fakes are seeded with the right state.
        var built = CreateHandler(watchZoneRepo, appRepo, notificationRepo, stateRepo);

        // Act
        var result = await built.HandleAsync(
            new GetApplicationsByZoneQuery("user-1", "zone-1"), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.Count).IsEqualTo(1);
        await Assert.That(result[0].LatestUnreadEvent).IsNotNull();
        await Assert.That(result[0].LatestUnreadEvent!.Type).IsEqualTo(NotificationEventType.DecisionUpdate);
        await Assert.That(result[0].LatestUnreadEvent!.Decision).IsEqualTo("Permitted");
        await Assert.That(result[0].LatestUnreadEvent!.CreatedAt).IsEqualTo(watermark.AddHours(1));
    }

    [Test]
    public async Task Should_LeaveLatestUnreadEventNull_When_AllNotificationsAtOrBeforeWatermark()
    {
        // Arrange — notification exists, but its CreatedAt is exactly the watermark.
        // Watermark is exclusive so the boundary instant counts as already read.
        var watermark = new DateTimeOffset(2026, 5, 1, 12, 0, 0, TimeSpan.Zero);

        var notificationRepo = new FakeNotificationRepository();
        var stateRepo = new FakeNotificationStateRepository();
        stateRepo.Seed(NotificationStateAggregate.Create("user-1", watermark));

        notificationRepo.Seed(BuildNotification(
            applicationUid: "uid-nearby",
            eventType: NotificationEventType.NewApplication,
            decision: null,
            createdAt: watermark));

        notificationRepo.Seed(BuildNotification(
            applicationUid: "uid-nearby",
            eventType: NotificationEventType.DecisionUpdate,
            decision: "Permitted",
            createdAt: watermark.AddSeconds(-30)));

        var watchZoneRepo = new FakeWatchZoneRepository();
        watchZoneRepo.Add(BuildZone());

        var appRepo = new FakePlanningApplicationRepository();
        await appRepo.UpsertAsync(
            new PlanningApplicationBuilder()
                .WithName("nearby-app")
                .WithUid("uid-nearby")
                .WithAreaId(42)
                .WithCoordinates(51.5380, -0.1410)
                .Build(),
            CancellationToken.None);

        var handler = CreateHandler(watchZoneRepo, appRepo, notificationRepo, stateRepo);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationsByZoneQuery("user-1", "zone-1"), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.Count).IsEqualTo(1);
        await Assert.That(result[0].LatestUnreadEvent).IsNull();
    }

    [Test]
    public async Task Should_LeaveLatestUnreadEventNull_When_UserHasNoState()
    {
        // Arrange — first-touch user with no NotificationState document yet. The
        // applications endpoint must NOT seed state (that's the dedicated
        // GET /me/notification-state endpoint's job). With no watermark we can't
        // safely classify anything as unread, so the field is null.
        var notificationRepo = new FakeNotificationRepository();
        var stateRepo = new FakeNotificationStateRepository();

        notificationRepo.Seed(BuildNotification(
            applicationUid: "uid-nearby",
            eventType: NotificationEventType.NewApplication,
            decision: null,
            createdAt: DateTimeOffset.UtcNow));

        var watchZoneRepo = new FakeWatchZoneRepository();
        watchZoneRepo.Add(BuildZone());

        var appRepo = new FakePlanningApplicationRepository();
        await appRepo.UpsertAsync(
            new PlanningApplicationBuilder()
                .WithName("nearby-app")
                .WithUid("uid-nearby")
                .WithAreaId(42)
                .WithCoordinates(51.5380, -0.1410)
                .Build(),
            CancellationToken.None);

        var handler = CreateHandler(watchZoneRepo, appRepo, notificationRepo, stateRepo);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationsByZoneQuery("user-1", "zone-1"), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.Count).IsEqualTo(1);
        await Assert.That(result[0].LatestUnreadEvent).IsNull();
        // And state must NOT have been seeded on the read-only path.
        await Assert.That(stateRepo.All).IsEmpty();
    }

    [Test]
    public async Task Should_PickMostRecentUnreadEvent_When_MultipleEventsForSameApplication()
    {
        // Arrange — two unread events (NewApplication + DecisionUpdate) for the
        // same application. The handler must surface the most recent one.
        var watermark = new DateTimeOffset(2026, 5, 1, 12, 0, 0, TimeSpan.Zero);

        var notificationRepo = new FakeNotificationRepository();
        var stateRepo = new FakeNotificationStateRepository();
        stateRepo.Seed(NotificationStateAggregate.Create("user-1", watermark));

        notificationRepo.Seed(BuildNotification(
            applicationUid: "uid-nearby",
            eventType: NotificationEventType.NewApplication,
            decision: null,
            createdAt: watermark.AddHours(1)));

        var laterCreated = watermark.AddHours(3);
        notificationRepo.Seed(BuildNotification(
            applicationUid: "uid-nearby",
            eventType: NotificationEventType.DecisionUpdate,
            decision: "Permitted",
            createdAt: laterCreated));

        var watchZoneRepo = new FakeWatchZoneRepository();
        watchZoneRepo.Add(BuildZone());

        var appRepo = new FakePlanningApplicationRepository();
        await appRepo.UpsertAsync(
            new PlanningApplicationBuilder()
                .WithName("nearby-app")
                .WithUid("uid-nearby")
                .WithAreaId(42)
                .WithCoordinates(51.5380, -0.1410)
                .Build(),
            CancellationToken.None);

        var handler = CreateHandler(watchZoneRepo, appRepo, notificationRepo, stateRepo);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationsByZoneQuery("user-1", "zone-1"), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result![0].LatestUnreadEvent).IsNotNull();
        await Assert.That(result[0].LatestUnreadEvent!.Type).IsEqualTo(NotificationEventType.DecisionUpdate);
        await Assert.That(result[0].LatestUnreadEvent!.CreatedAt).IsEqualTo(laterCreated);
    }

    private static (GetApplicationsByZoneQueryHandler Handler,
        FakeNotificationRepository NotificationRepo,
        FakeNotificationStateRepository StateRepo) CreateBuilt()
    {
        var notificationRepo = new FakeNotificationRepository();
        var stateRepo = new FakeNotificationStateRepository();
        var watchZoneRepo = new FakeWatchZoneRepository();
        var appRepo = new FakePlanningApplicationRepository();

        var handler = new GetApplicationsByZoneQueryHandler(
            watchZoneRepo, appRepo, notificationRepo, stateRepo);
        return (handler, notificationRepo, stateRepo);
    }

    private static GetApplicationsByZoneQueryHandler CreateHandler(
        FakeWatchZoneRepository watchZoneRepo,
        FakePlanningApplicationRepository appRepo)
    {
        return new GetApplicationsByZoneQueryHandler(
            watchZoneRepo,
            appRepo,
            new FakeNotificationRepository(),
            new FakeNotificationStateRepository());
    }

    private static GetApplicationsByZoneQueryHandler CreateHandler(
        FakeWatchZoneRepository watchZoneRepo,
        FakePlanningApplicationRepository appRepo,
        FakeNotificationRepository notificationRepo,
        FakeNotificationStateRepository stateRepo)
    {
        return new GetApplicationsByZoneQueryHandler(
            watchZoneRepo, appRepo, notificationRepo, stateRepo);
    }

    private static void SeedZone(FakeNotificationStateRepository stateRepo, DateTimeOffset watermark)
    {
        stateRepo.Seed(NotificationStateAggregate.Create("user-1", watermark));
    }

    private static Domain.WatchZones.WatchZone BuildZone()
    {
        return new WatchZoneBuilder()
            .WithId("zone-1")
            .WithUserId("user-1")
            .WithName("My Zone")
            .WithCentre(51.5390, -0.1426)
            .WithRadiusMetres(1000)
            .WithAuthorityId(42)
            .Build();
    }

    private static Notification BuildNotification(
        string applicationUid,
        NotificationEventType eventType,
        string? decision,
        DateTimeOffset createdAt)
    {
        return Notification.Create(
            userId: "user-1",
            applicationUid: applicationUid,
            applicationName: "app-name",
            watchZoneId: "zone-1",
            applicationAddress: "addr",
            applicationDescription: "desc",
            applicationType: "Full",
            authorityId: 42,
            now: createdAt,
            decision: decision,
            eventType: eventType);
    }
}
