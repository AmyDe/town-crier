using TownCrier.Application.DecisionAlerts;
using TownCrier.Application.Tests.Notifications;
using TownCrier.Application.Tests.Polling;
using TownCrier.Application.Tests.SavedApplications;
using TownCrier.Application.Tests.UserProfiles;
using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Domain.SavedApplications;
using TownCrier.Domain.UserProfiles;
using FakeDeviceRegistrationRepository = TownCrier.Application.Tests.DeviceRegistrations.FakeDeviceRegistrationRepository;

namespace TownCrier.Application.Tests.DecisionAlerts;

public sealed class DispatchDecisionAlertCommandHandlerTests
{
    private static readonly DateTimeOffset March2026 = new(2026, 3, 15, 10, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task Should_CreateDecisionAlert_When_BookmarkedApplicationReceivesDecision()
    {
        // Arrange
        var (handler, alertRepo, savedAppRepo, userProfileRepo, _, deviceRepo) = CreateHandler();
        await SeedBookmarkedUserWithDevice(savedAppRepo, userProfileRepo, deviceRepo);

        var command = CreateCommand();

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(alertRepo.All).HasCount().EqualTo(1);
        await Assert.That(alertRepo.All[0].UserId).IsEqualTo("user-1");
        await Assert.That(alertRepo.All[0].ApplicationUid).IsEqualTo("test-uid-001");
        await Assert.That(alertRepo.All[0].Decision).IsEqualTo("Approved");
    }

    [Test]
    public async Task Should_SendPushNotification_When_UserHasRegisteredDevice()
    {
        // Arrange
        var (handler, _, savedAppRepo, userProfileRepo, pushSender, deviceRepo) = CreateHandler();
        await SeedBookmarkedUserWithDevice(savedAppRepo, userProfileRepo, deviceRepo);

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert
        await Assert.That(pushSender.DecisionAlertsSent).HasCount().EqualTo(1);
        await Assert.That(pushSender.DecisionAlertsSent[0].Devices).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_MarkPushSent_When_AlertDispatched()
    {
        // Arrange
        var (handler, alertRepo, savedAppRepo, userProfileRepo, _, deviceRepo) = CreateHandler();
        await SeedBookmarkedUserWithDevice(savedAppRepo, userProfileRepo, deviceRepo);

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert
        await Assert.That(alertRepo.All[0].PushSent).IsTrue();
    }

    [Test]
    public async Task Should_NotCreateDuplicateAlert_When_AlreadyAlerted()
    {
        // Arrange
        var (handler, alertRepo, savedAppRepo, userProfileRepo, pushSender, deviceRepo) = CreateHandler();
        await SeedBookmarkedUserWithDevice(savedAppRepo, userProfileRepo, deviceRepo);

        var command = CreateCommand();

        // Act
        await handler.HandleAsync(command, CancellationToken.None);
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(alertRepo.All).HasCount().EqualTo(1);
        await Assert.That(pushSender.DecisionAlertsSent).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_AlertMultipleUsers_When_MultipleUsersBookmarked()
    {
        // Arrange
        var (handler, alertRepo, savedAppRepo, userProfileRepo, pushSender, deviceRepo) = CreateHandler();
        await SeedBookmarkedUserWithDevice(savedAppRepo, userProfileRepo, deviceRepo, "user-1");
        await SeedBookmarkedUserWithDevice(savedAppRepo, userProfileRepo, deviceRepo, "user-2");

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert
        await Assert.That(alertRepo.All).HasCount().EqualTo(2);
        await Assert.That(pushSender.DecisionAlertsSent).HasCount().EqualTo(2);
    }

    [Test]
    public async Task Should_RecordAlertButNotPush_When_PushDisabled()
    {
        // Arrange
        var (handler, alertRepo, savedAppRepo, userProfileRepo, pushSender, _) = CreateHandler();

        var savedApp = SavedApplication.Create("user-1", "test-uid-001", March2026);
        await savedAppRepo.SaveAsync(savedApp, CancellationToken.None);

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithPushEnabled(false)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert
        await Assert.That(alertRepo.All).HasCount().EqualTo(1);
        await Assert.That(alertRepo.All[0].PushSent).IsFalse();
        await Assert.That(pushSender.DecisionAlertsSent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_NotCreateAlert_When_UserProfileNotFound()
    {
        // Arrange — bookmark exists but no user profile
        var (handler, alertRepo, savedAppRepo, _, pushSender, _) = CreateHandler();

        var savedApp = SavedApplication.Create("user-1", "test-uid-001", March2026);
        await savedAppRepo.SaveAsync(savedApp, CancellationToken.None);

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert
        await Assert.That(alertRepo.All).HasCount().EqualTo(0);
        await Assert.That(pushSender.DecisionAlertsSent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_NotSendPush_When_NoRegisteredDevices()
    {
        // Arrange
        var (handler, alertRepo, savedAppRepo, userProfileRepo, pushSender, _) = CreateHandler();

        var savedApp = SavedApplication.Create("user-1", "test-uid-001", March2026);
        await savedAppRepo.SaveAsync(savedApp, CancellationToken.None);

        var profile = new UserProfileBuilder().WithUserId("user-1").Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert
        await Assert.That(alertRepo.All).HasCount().EqualTo(1);
        await Assert.That(alertRepo.All[0].PushSent).IsFalse();
        await Assert.That(pushSender.DecisionAlertsSent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_NotCreateAlert_When_NoUsersBookmarked()
    {
        // Arrange — no bookmarks
        var (handler, alertRepo, _, _, pushSender, _) = CreateHandler();

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert
        await Assert.That(alertRepo.All).HasCount().EqualTo(0);
        await Assert.That(pushSender.DecisionAlertsSent).HasCount().EqualTo(0);
    }

    private static (DispatchDecisionAlertCommandHandler Handler,
        FakeDecisionAlertRepository AlertRepo,
        FakeSavedApplicationRepository SavedAppRepo,
        FakeUserProfileRepository UserProfileRepo,
        SpyDecisionAlertPushSender PushSender,
        FakeDeviceRegistrationRepository DeviceRepo) CreateHandler(FakeTimeProvider? timeProvider = null)
    {
        var alertRepo = new FakeDecisionAlertRepository();
        var savedAppRepo = new FakeSavedApplicationRepository();
        var userProfileRepo = new FakeUserProfileRepository();
        var deviceRepo = new FakeDeviceRegistrationRepository();
        var pushSender = new SpyDecisionAlertPushSender();
        var tp = timeProvider ?? new FakeTimeProvider(March2026);

        var handler = new DispatchDecisionAlertCommandHandler(
            alertRepo, savedAppRepo, userProfileRepo, deviceRepo, pushSender, tp);

        return (handler, alertRepo, savedAppRepo, userProfileRepo, pushSender, deviceRepo);
    }

    private static async Task SeedBookmarkedUserWithDevice(
        FakeSavedApplicationRepository savedAppRepo,
        FakeUserProfileRepository userProfileRepo,
        FakeDeviceRegistrationRepository deviceRepo,
        string userId = "user-1")
    {
        var savedApp = SavedApplication.Create(userId, "test-uid-001", March2026);
        await savedAppRepo.SaveAsync(savedApp, CancellationToken.None);

        var profile = new UserProfileBuilder()
            .WithUserId(userId)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var device = DeviceRegistration.Create(userId, $"device-token-{userId}", DevicePlatform.Ios, March2026);
        await deviceRepo.SaveAsync(device, CancellationToken.None);
    }

    private static DispatchDecisionAlertCommand CreateCommand(string appState = "Approved")
    {
        var application = new PlanningApplicationBuilder()
            .WithUid("test-uid-001")
            .WithName("Test Application")
            .WithAppState(appState)
            .WithCoordinates(51.5074, -0.1278)
            .Build();

        return new DispatchDecisionAlertCommand(application);
    }
}
