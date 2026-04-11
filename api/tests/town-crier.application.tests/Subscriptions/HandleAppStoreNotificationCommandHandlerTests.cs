using TownCrier.Application.Subscriptions;
using TownCrier.Application.Tests.Admin;
using TownCrier.Application.Tests.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.Subscriptions;

public sealed class HandleAppStoreNotificationCommandHandlerTests
{
    private const string SignedPayload = "outer.payload.signature";
    private const string OuterJson = "{\"outer\":true}";
    private const string SignedTransactionInfo = "inner.txn.signature";
    private const string TxnJson = "{\"txn\":true}";
    private const string NotificationUuid = "notif-uuid-1";
    private const string OriginalTxnId = "orig-txn-1";
    private const string UserId = "auth0|user-1";

    [Test]
    public async Task Should_ActivateSubscription_When_InitialBuyNotification()
    {
        // Arrange
        var deps = CreateDependencies();
        var profile = CreateProfileWithTransaction(deps.Repository);
        SetupNotification(deps, "SUBSCRIBED", "INITIAL_BUY");

        var handler = CreateHandler(deps);
        var command = new HandleAppStoreNotificationCommand(SignedPayload);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var saved = deps.Repository.GetByUserId(UserId);
        await Assert.That(saved!.Tier).IsEqualTo(SubscriptionTier.Personal);
    }

    [Test]
    public async Task Should_ActivateSubscription_When_ResubscribeNotification()
    {
        // Arrange
        var deps = CreateDependencies();
        var profile = CreateProfileWithTransaction(deps.Repository);
        SetupNotification(deps, "SUBSCRIBED", "RESUBSCRIBE");

        var handler = CreateHandler(deps);
        var command = new HandleAppStoreNotificationCommand(SignedPayload);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var saved = deps.Repository.GetByUserId(UserId);
        await Assert.That(saved!.Tier).IsEqualTo(SubscriptionTier.Personal);
    }

    [Test]
    public async Task Should_RenewSubscription_When_DidRenewNotification()
    {
        // Arrange
        var deps = CreateDependencies();
        var profile = CreateProfileWithTransaction(deps.Repository);
        profile.ActivateSubscription(
            SubscriptionTier.Personal,
            new DateTimeOffset(2026, 4, 11, 0, 0, 0, TimeSpan.Zero));
        await deps.Repository.SaveAsync(profile, CancellationToken.None);

        SetupNotification(deps, "DID_RENEW", null);

        var handler = CreateHandler(deps);
        var command = new HandleAppStoreNotificationCommand(SignedPayload);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var saved = deps.Repository.GetByUserId(UserId);
        await Assert.That(saved!.SubscriptionExpiry)
            .IsEqualTo(new DateTimeOffset(2026, 5, 11, 0, 0, 0, TimeSpan.Zero));
    }

    [Test]
    public async Task Should_ExpireSubscription_When_ExpiredNotification()
    {
        // Arrange
        var deps = CreateDependencies();
        var profile = CreateProfileWithTransaction(deps.Repository);
        profile.ActivateSubscription(
            SubscriptionTier.Personal,
            new DateTimeOffset(2026, 4, 11, 0, 0, 0, TimeSpan.Zero));
        await deps.Repository.SaveAsync(profile, CancellationToken.None);

        SetupNotification(deps, "EXPIRED", null);

        var handler = CreateHandler(deps);
        var command = new HandleAppStoreNotificationCommand(SignedPayload);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var saved = deps.Repository.GetByUserId(UserId);
        await Assert.That(saved!.Tier).IsEqualTo(SubscriptionTier.Free);
    }

    [Test]
    public async Task Should_EnterGracePeriod_When_DidFailToRenewWithGracePeriod()
    {
        // Arrange
        var deps = CreateDependencies();
        var profile = CreateProfileWithTransaction(deps.Repository);
        profile.ActivateSubscription(
            SubscriptionTier.Personal,
            new DateTimeOffset(2026, 4, 11, 0, 0, 0, TimeSpan.Zero));
        await deps.Repository.SaveAsync(profile, CancellationToken.None);

        var gracePeriodExpiry = new DateTimeOffset(2026, 4, 27, 0, 0, 0, TimeSpan.Zero);
        SetupNotification(deps, "DID_FAIL_TO_RENEW", "GRACE_PERIOD", gracePeriodExpiresDate: gracePeriodExpiry);

        var handler = CreateHandler(deps);
        var command = new HandleAppStoreNotificationCommand(SignedPayload);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var saved = deps.Repository.GetByUserId(UserId);
        await Assert.That(saved!.GracePeriodExpiry).IsEqualTo(gracePeriodExpiry);
    }

    [Test]
    public async Task Should_ExpireSubscription_When_DidFailToRenewWithoutGracePeriod()
    {
        // Arrange
        var deps = CreateDependencies();
        var profile = CreateProfileWithTransaction(deps.Repository);
        profile.ActivateSubscription(
            SubscriptionTier.Personal,
            new DateTimeOffset(2026, 4, 11, 0, 0, 0, TimeSpan.Zero));
        await deps.Repository.SaveAsync(profile, CancellationToken.None);

        SetupNotification(deps, "DID_FAIL_TO_RENEW", null);

        var handler = CreateHandler(deps);
        var command = new HandleAppStoreNotificationCommand(SignedPayload);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var saved = deps.Repository.GetByUserId(UserId);
        await Assert.That(saved!.Tier).IsEqualTo(SubscriptionTier.Free);
    }

    [Test]
    public async Task Should_ExpireSubscription_When_RefundNotification()
    {
        // Arrange
        var deps = CreateDependencies();
        var profile = CreateProfileWithTransaction(deps.Repository);
        profile.ActivateSubscription(
            SubscriptionTier.Personal,
            new DateTimeOffset(2026, 4, 11, 0, 0, 0, TimeSpan.Zero));
        await deps.Repository.SaveAsync(profile, CancellationToken.None);

        SetupNotification(deps, "REFUND", null);

        var handler = CreateHandler(deps);
        var command = new HandleAppStoreNotificationCommand(SignedPayload);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var saved = deps.Repository.GetByUserId(UserId);
        await Assert.That(saved!.Tier).IsEqualTo(SubscriptionTier.Free);
    }

    [Test]
    public async Task Should_ReturnImmediately_When_NotificationAlreadyProcessed()
    {
        // Arrange
        var deps = CreateDependencies();
        deps.IdempotencyStore.SeedProcessed(NotificationUuid);
        SetupNotification(deps, "SUBSCRIBED", "INITIAL_BUY");

        // No profile -- if handler proceeds past idempotency check, it will fail
        var handler = CreateHandler(deps);
        var command = new HandleAppStoreNotificationCommand(SignedPayload);

        // Act -- should not throw
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert -- no new processed entries (already existed)
        await Assert.That(deps.IdempotencyStore.MarkedProcessed).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_MarkNotificationProcessed_When_HandledSuccessfully()
    {
        // Arrange
        var deps = CreateDependencies();
        CreateProfileWithTransaction(deps.Repository);
        SetupNotification(deps, "SUBSCRIBED", "INITIAL_BUY");

        var handler = CreateHandler(deps);
        var command = new HandleAppStoreNotificationCommand(SignedPayload);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(deps.IdempotencyStore.MarkedProcessed).Contains(NotificationUuid);
    }

    [Test]
    public async Task Should_SyncTierToAuth0_When_SubscriptionActivated()
    {
        // Arrange
        var deps = CreateDependencies();
        CreateProfileWithTransaction(deps.Repository);
        SetupNotification(deps, "SUBSCRIBED", "INITIAL_BUY");

        var handler = CreateHandler(deps);
        var command = new HandleAppStoreNotificationCommand(SignedPayload);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(deps.Auth0.Updates).HasCount().EqualTo(1);
        await Assert.That(deps.Auth0.Updates[0].Tier).IsEqualTo("Personal");
    }

    [Test]
    public async Task Should_NotThrow_When_ProfileNotFoundForTransaction()
    {
        // Arrange
        var deps = CreateDependencies();
        SetupNotification(deps, "SUBSCRIBED", "INITIAL_BUY");

        // No profile with this original transaction ID
        var handler = CreateHandler(deps);
        var command = new HandleAppStoreNotificationCommand(SignedPayload);

        // Act -- should not throw, just return
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert -- notification still marked processed
        await Assert.That(deps.IdempotencyStore.MarkedProcessed).Contains(NotificationUuid);
    }

    [Test]
    public async Task Should_NotModifyProfile_When_TestNotification()
    {
        // Arrange
        var deps = CreateDependencies();
        var profile = CreateProfileWithTransaction(deps.Repository);
        SetupNotification(deps, "TEST", null);

        var handler = CreateHandler(deps);
        var command = new HandleAppStoreNotificationCommand(SignedPayload);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var saved = deps.Repository.GetByUserId(UserId);
        await Assert.That(saved!.Tier).IsEqualTo(SubscriptionTier.Free);
        await Assert.That(deps.Auth0.Updates).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_ActivateSubscription_When_UpgradeNotification()
    {
        // Arrange
        var deps = CreateDependencies();
        var profile = CreateProfileWithTransaction(deps.Repository);
        profile.ActivateSubscription(
            SubscriptionTier.Personal,
            new DateTimeOffset(2026, 5, 11, 0, 0, 0, TimeSpan.Zero));
        await deps.Repository.SaveAsync(profile, CancellationToken.None);

        SetupNotification(
            deps,
            "DID_CHANGE_RENEWAL_PREF",
            "UPGRADE",
            productId: "uk.co.towncrier.pro.monthly");

        var handler = CreateHandler(deps);
        var command = new HandleAppStoreNotificationCommand(SignedPayload);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var saved = deps.Repository.GetByUserId(UserId);
        await Assert.That(saved!.Tier).IsEqualTo(SubscriptionTier.Pro);
    }

    [Test]
    public async Task Should_NotChangeState_When_DowngradeNotification()
    {
        // Arrange
        var deps = CreateDependencies();
        var profile = CreateProfileWithTransaction(deps.Repository);
        profile.ActivateSubscription(
            SubscriptionTier.Pro,
            new DateTimeOffset(2026, 5, 11, 0, 0, 0, TimeSpan.Zero));
        await deps.Repository.SaveAsync(profile, CancellationToken.None);

        SetupNotification(deps, "DID_CHANGE_RENEWAL_PREF", "DOWNGRADE");

        var handler = CreateHandler(deps);
        var command = new HandleAppStoreNotificationCommand(SignedPayload);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert -- tier stays Pro (downgrade takes effect at renewal)
        var saved = deps.Repository.GetByUserId(UserId);
        await Assert.That(saved!.Tier).IsEqualTo(SubscriptionTier.Pro);
    }

    [Test]
    public async Task Should_ExpireSubscription_When_RevokeNotification()
    {
        // Arrange
        var deps = CreateDependencies();
        var profile = CreateProfileWithTransaction(deps.Repository);
        profile.ActivateSubscription(
            SubscriptionTier.Personal,
            new DateTimeOffset(2026, 5, 11, 0, 0, 0, TimeSpan.Zero));
        await deps.Repository.SaveAsync(profile, CancellationToken.None);

        SetupNotification(deps, "REVOKE", null);

        var handler = CreateHandler(deps);
        var command = new HandleAppStoreNotificationCommand(SignedPayload);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var saved = deps.Repository.GetByUserId(UserId);
        await Assert.That(saved!.Tier).IsEqualTo(SubscriptionTier.Free);
    }

    [Test]
    public async Task Should_ExpireSubscription_When_GracePeriodExpiredNotification()
    {
        // Arrange
        var deps = CreateDependencies();
        var profile = CreateProfileWithTransaction(deps.Repository);
        profile.ActivateSubscription(
            SubscriptionTier.Personal,
            new DateTimeOffset(2026, 4, 11, 0, 0, 0, TimeSpan.Zero));
        profile.EnterGracePeriod(new DateTimeOffset(2026, 4, 27, 0, 0, 0, TimeSpan.Zero));
        await deps.Repository.SaveAsync(profile, CancellationToken.None);

        SetupNotification(deps, "GRACE_PERIOD_EXPIRED", null);

        var handler = CreateHandler(deps);
        var command = new HandleAppStoreNotificationCommand(SignedPayload);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var saved = deps.Repository.GetByUserId(UserId);
        await Assert.That(saved!.Tier).IsEqualTo(SubscriptionTier.Free);
    }

    [Test]
    public async Task Should_ActivateSubscription_When_OfferRedeemedNotification()
    {
        // Arrange
        var deps = CreateDependencies();
        CreateProfileWithTransaction(deps.Repository);
        SetupNotification(deps, "OFFER_REDEEMED", null);

        var handler = CreateHandler(deps);
        var command = new HandleAppStoreNotificationCommand(SignedPayload);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var saved = deps.Repository.GetByUserId(UserId);
        await Assert.That(saved!.Tier).IsEqualTo(SubscriptionTier.Personal);
    }

    private static UserProfile CreateProfileWithTransaction(FakeUserProfileRepository repository)
    {
        var profile = UserProfile.Register(UserId);
        profile.LinkOriginalTransactionId(OriginalTxnId);
        repository.SaveAsync(profile, CancellationToken.None).GetAwaiter().GetResult();
        return profile;
    }

    private static void SetupNotification(
        TestDependencies deps,
        string notificationType,
        string? subtype,
        string productId = "uk.co.towncrier.personal.monthly",
        DateTimeOffset? gracePeriodExpiresDate = null)
    {
        var notification = new DecodedNotification(
            NotificationType: notificationType,
            Subtype: subtype,
            NotificationUuid: NotificationUuid,
            SignedTransactionInfo: SignedTransactionInfo,
            SignedRenewalInfo: null);

        deps.NotificationDecoder.Register(OuterJson, notification);

        var transaction = new DecodedTransaction(
            TransactionId: "txn-1",
            OriginalTransactionId: OriginalTxnId,
            ProductId: productId,
            BundleId: "uk.co.towncrier.ios",
            PurchaseDate: new DateTimeOffset(2026, 4, 11, 0, 0, 0, TimeSpan.Zero),
            ExpiresDate: new DateTimeOffset(2026, 5, 11, 0, 0, 0, TimeSpan.Zero),
            Environment: "Production");

        deps.TransactionDecoder.Register(TxnJson, transaction);

        if (gracePeriodExpiresDate.HasValue)
        {
            deps.GracePeriodExpiresDate = gracePeriodExpiresDate.Value;
        }
    }

    private static HandleAppStoreNotificationCommandHandler CreateHandler(TestDependencies deps)
    {
        return new HandleAppStoreNotificationCommandHandler(
            deps.Verifier,
            deps.NotificationDecoder,
            deps.TransactionDecoder,
            deps.Repository,
            deps.Auth0,
            deps.IdempotencyStore,
            deps.Settings);
    }

    private static TestDependencies CreateDependencies()
    {
        var verifier = new FakeAppleJwsVerifier();
        verifier.SetPayload(SignedPayload, OuterJson);
        verifier.SetPayload(SignedTransactionInfo, TxnJson);

        return new TestDependencies
        {
            Verifier = verifier,
            NotificationDecoder = new FakeNotificationDecoder(),
            TransactionDecoder = new FakeTransactionDecoder(),
            Repository = new FakeUserProfileRepository(),
            Auth0 = new FakeAuth0ManagementClient(),
            IdempotencyStore = new FakeNotificationIdempotencyStore(),
            Settings = new AppleSettings
            {
                BundleId = "uk.co.towncrier.ios",
                Environment = "Production",
            },
        };
    }

    private sealed class TestDependencies
    {
        public required FakeAppleJwsVerifier Verifier { get; init; }

        public required FakeNotificationDecoder NotificationDecoder { get; init; }

        public required FakeTransactionDecoder TransactionDecoder { get; init; }

        public required FakeUserProfileRepository Repository { get; init; }

        public required FakeAuth0ManagementClient Auth0 { get; init; }

        public required FakeNotificationIdempotencyStore IdempotencyStore { get; init; }

        public required AppleSettings Settings { get; init; }

        public DateTimeOffset GracePeriodExpiresDate { get; set; }
    }
}
