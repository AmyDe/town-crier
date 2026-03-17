using TownCrier.Application.Subscriptions;
using TownCrier.Application.Tests.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.Subscriptions;

public sealed class HandleAppStoreNotificationCommandHandlerTests
{
    [Test]
    public async Task Should_ReturnInvalidSignature_When_PayloadFailsValidation()
    {
        // Arrange
        var validator = new FakeAppStoreNotificationValidator();
        var repository = new FakeUserProfileRepository();
        var handler = new HandleAppStoreNotificationCommandHandler(validator, repository);

        var command = new HandleAppStoreNotificationCommand("invalid-jws-payload");

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Outcome).IsEqualTo(NotificationOutcome.InvalidSignature);
    }

    [Test]
    public async Task Should_ReturnUserNotFound_When_NoUserMatchesOriginalTransactionId()
    {
        // Arrange
        var validator = new FakeAppStoreNotificationValidator();
        validator.AddValidPayload("valid-jws", new AppStoreNotification(
            AppStoreNotificationType.DidRenew,
            OriginalTransactionId: "txn-unknown",
            Tier: SubscriptionTier.Pro,
            ExpiresDate: new DateTimeOffset(2026, 4, 16, 0, 0, 0, TimeSpan.Zero)));

        var repository = new FakeUserProfileRepository();
        var handler = new HandleAppStoreNotificationCommandHandler(validator, repository);

        var command = new HandleAppStoreNotificationCommand("valid-jws");

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Outcome).IsEqualTo(NotificationOutcome.UserNotFound);
    }

    [Test]
    public async Task Should_ActivateSubscription_When_SubscribedNotificationReceived()
    {
        // Arrange
        var validator = new FakeAppStoreNotificationValidator();
        validator.AddValidPayload("subscribed-jws", new AppStoreNotification(
            AppStoreNotificationType.Subscribed,
            OriginalTransactionId: "txn-100",
            Tier: SubscriptionTier.Pro,
            ExpiresDate: new DateTimeOffset(2026, 4, 16, 0, 0, 0, TimeSpan.Zero)));

        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-1");
        profile.LinkOriginalTransactionId("txn-100");
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new HandleAppStoreNotificationCommandHandler(validator, repository);
        var command = new HandleAppStoreNotificationCommand("subscribed-jws");

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Outcome).IsEqualTo(NotificationOutcome.Processed);
        var saved = repository.GetByUserId("auth0|user-1");
        await Assert.That(saved!.Tier).IsEqualTo(SubscriptionTier.Pro);
        await Assert.That(saved.SubscriptionExpiry).IsEqualTo(new DateTimeOffset(2026, 4, 16, 0, 0, 0, TimeSpan.Zero));
    }

    [Test]
    public async Task Should_ExtendExpiryAndClearGracePeriod_When_DidRenewNotificationReceived()
    {
        // Arrange
        var validator = new FakeAppStoreNotificationValidator();
        validator.AddValidPayload("renew-jws", new AppStoreNotification(
            AppStoreNotificationType.DidRenew,
            OriginalTransactionId: "txn-200",
            Tier: SubscriptionTier.Pro,
            ExpiresDate: new DateTimeOffset(2026, 5, 16, 0, 0, 0, TimeSpan.Zero)));

        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-2");
        profile.LinkOriginalTransactionId("txn-200");
        profile.ActivateSubscription(SubscriptionTier.Pro, new DateTimeOffset(2026, 4, 16, 0, 0, 0, TimeSpan.Zero));
        profile.EnterGracePeriod(new DateTimeOffset(2026, 4, 30, 0, 0, 0, TimeSpan.Zero));
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new HandleAppStoreNotificationCommandHandler(validator, repository);
        var command = new HandleAppStoreNotificationCommand("renew-jws");

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Outcome).IsEqualTo(NotificationOutcome.Processed);
        var saved = repository.GetByUserId("auth0|user-2");
        await Assert.That(saved!.SubscriptionExpiry).IsEqualTo(new DateTimeOffset(2026, 5, 16, 0, 0, 0, TimeSpan.Zero));
        await Assert.That(saved.GracePeriodExpiry).IsNull();
    }

    [Test]
    public async Task Should_RevertToFree_When_ExpiredNotificationReceived()
    {
        // Arrange
        var validator = new FakeAppStoreNotificationValidator();
        validator.AddValidPayload("expired-jws", new AppStoreNotification(
            AppStoreNotificationType.Expired,
            OriginalTransactionId: "txn-300",
            Tier: SubscriptionTier.Pro,
            ExpiresDate: null));

        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-3");
        profile.LinkOriginalTransactionId("txn-300");
        profile.ActivateSubscription(SubscriptionTier.Pro, new DateTimeOffset(2026, 4, 16, 0, 0, 0, TimeSpan.Zero));
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new HandleAppStoreNotificationCommandHandler(validator, repository);
        var command = new HandleAppStoreNotificationCommand("expired-jws");

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Outcome).IsEqualTo(NotificationOutcome.Processed);
        var saved = repository.GetByUserId("auth0|user-3");
        await Assert.That(saved!.Tier).IsEqualTo(SubscriptionTier.Free);
        await Assert.That(saved.SubscriptionExpiry).IsNull();
    }

    [Test]
    public async Task Should_RevertToFreeImmediately_When_RefundNotificationReceived()
    {
        // Arrange
        var validator = new FakeAppStoreNotificationValidator();
        validator.AddValidPayload("refund-jws", new AppStoreNotification(
            AppStoreNotificationType.Refund,
            OriginalTransactionId: "txn-400",
            Tier: SubscriptionTier.Pro,
            ExpiresDate: null));

        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-4");
        profile.LinkOriginalTransactionId("txn-400");
        profile.ActivateSubscription(SubscriptionTier.Pro, new DateTimeOffset(2026, 4, 16, 0, 0, 0, TimeSpan.Zero));
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new HandleAppStoreNotificationCommandHandler(validator, repository);
        var command = new HandleAppStoreNotificationCommand("refund-jws");

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Outcome).IsEqualTo(NotificationOutcome.Processed);
        var saved = repository.GetByUserId("auth0|user-4");
        await Assert.That(saved!.Tier).IsEqualTo(SubscriptionTier.Free);
        await Assert.That(saved.SubscriptionExpiry).IsNull();
    }

    [Test]
    public async Task Should_RevertToFree_When_GracePeriodExpiredNotificationReceived()
    {
        // Arrange
        var validator = new FakeAppStoreNotificationValidator();
        validator.AddValidPayload("grace-expired-jws", new AppStoreNotification(
            AppStoreNotificationType.GracePeriodExpired,
            OriginalTransactionId: "txn-500",
            Tier: SubscriptionTier.Pro,
            ExpiresDate: null));

        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-5");
        profile.LinkOriginalTransactionId("txn-500");
        profile.ActivateSubscription(SubscriptionTier.Pro, new DateTimeOffset(2026, 4, 16, 0, 0, 0, TimeSpan.Zero));
        profile.EnterGracePeriod(new DateTimeOffset(2026, 4, 30, 0, 0, 0, TimeSpan.Zero));
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new HandleAppStoreNotificationCommandHandler(validator, repository);
        var command = new HandleAppStoreNotificationCommand("grace-expired-jws");

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Outcome).IsEqualTo(NotificationOutcome.Processed);
        var saved = repository.GetByUserId("auth0|user-5");
        await Assert.That(saved!.Tier).IsEqualTo(SubscriptionTier.Free);
        await Assert.That(saved.GracePeriodExpiry).IsNull();
    }
}
