using TownCrier.Application.Subscriptions;
using TownCrier.Application.Tests.Admin;
using TownCrier.Application.Tests.UserProfiles;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.Subscriptions;

public sealed class VerifySubscriptionCommandHandlerTests
{
    private const string SignedTransaction = "header.payload.signature";
    private const string DecodedJson = "{\"decoded\":true}";
    private const string BundleId = "uk.co.towncrier.ios";
    private const string UserId = "auth0|user-1";

    // Expiry instants used by restore tests. Active/ActiveLater are far in the
    // future; Expired is in the past. The restore tests run wall-clock, so the
    // gap to "now" must be generous.
    private static readonly DateTimeOffset Active = DateTimeOffset.UtcNow.AddDays(30);
    private static readonly DateTimeOffset ActiveLater = DateTimeOffset.UtcNow.AddDays(60);
    private static readonly DateTimeOffset Expired = DateTimeOffset.UtcNow.AddDays(-30);

    // JWS lists for the restore tests, hoisted to satisfy CA1861.
    private static readonly string[] SingleRestoreJws = ["restore.jws.1"];
    private static readonly string[] PersonalAndExpiredProJws = ["restore.personal", "restore.expired-pro"];
    private static readonly string[] PersonalAndProJws = ["restore.personal", "restore.pro"];
    private static readonly string[] ExpiredOnlyJws = ["restore.expired"];
    private static readonly string[] ValidAndTamperedJws = ["restore.valid", "restore.tampered"];

    [Test]
    public async Task Should_ActivatePersonalSubscription_When_ValidTransaction()
    {
        // Arrange
        var (verifier, decoder, repository, auth0, settings) = CreateDependencies();
        var transaction = CreateTransaction();
        decoder.Register(DecodedJson, transaction);

        var profile = UserProfile.Register(UserId);
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new VerifySubscriptionCommandHandler(
            verifier, decoder, repository, auth0, settings);
        var command = new VerifySubscriptionCommand(UserId, SignedTransaction);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Personal);
        await Assert.That(result.SubscriptionExpiry).IsEqualTo(transaction.ExpiresDate);
    }

    [Test]
    public async Task Should_LinkOriginalTransactionId_When_VerifySucceeds()
    {
        // Arrange
        var (verifier, decoder, repository, auth0, settings) = CreateDependencies();
        decoder.Register(DecodedJson, CreateTransaction());

        var profile = UserProfile.Register(UserId);
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new VerifySubscriptionCommandHandler(
            verifier, decoder, repository, auth0, settings);
        var command = new VerifySubscriptionCommand(UserId, SignedTransaction);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var saved = repository.GetByUserId(UserId);
        await Assert.That(saved!.OriginalTransactionId).IsEqualTo("orig-txn-1");
    }

    [Test]
    public async Task Should_SyncTierToAuth0_When_VerifySucceeds()
    {
        // Arrange
        var (verifier, decoder, repository, auth0, settings) = CreateDependencies();
        decoder.Register(DecodedJson, CreateTransaction());

        var profile = UserProfile.Register(UserId);
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new VerifySubscriptionCommandHandler(
            verifier, decoder, repository, auth0, settings);
        var command = new VerifySubscriptionCommand(UserId, SignedTransaction);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(auth0.Updates).HasCount().EqualTo(1);
        await Assert.That(auth0.Updates[0].Tier).IsEqualTo("Personal");
    }

    [Test]
    public async Task Should_ActivateProSubscription_When_ProProductId()
    {
        // Arrange
        var (verifier, decoder, repository, auth0, settings) = CreateDependencies();
        var transaction = CreateTransaction(productId: "uk.co.towncrier.pro.monthly");
        decoder.Register(DecodedJson, transaction);

        var profile = UserProfile.Register(UserId);
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new VerifySubscriptionCommandHandler(
            verifier, decoder, repository, auth0, settings);
        var command = new VerifySubscriptionCommand(UserId, SignedTransaction);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Pro);
    }

    [Test]
    public async Task Should_ThrowAppleJwsVerificationException_When_JwsInvalid()
    {
        // Arrange
        var (verifier, decoder, repository, auth0, settings) = CreateDependencies();
        verifier.SetShouldFail();

        var profile = UserProfile.Register(UserId);
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new VerifySubscriptionCommandHandler(
            verifier, decoder, repository, auth0, settings);
        var command = new VerifySubscriptionCommand(UserId, SignedTransaction);

        // Act & Assert
        await Assert.ThrowsAsync<AppleJwsVerificationException>(
            () => handler.HandleAsync(command, CancellationToken.None));
    }

    [Test]
    public async Task Should_ThrowArgumentException_When_BundleIdMismatch()
    {
        // Arrange
        var (verifier, decoder, repository, auth0, settings) = CreateDependencies();
        var transaction = CreateTransaction(bundleId: "com.evil.app");
        decoder.Register(DecodedJson, transaction);

        var profile = UserProfile.Register(UserId);
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new VerifySubscriptionCommandHandler(
            verifier, decoder, repository, auth0, settings);
        var command = new VerifySubscriptionCommand(UserId, SignedTransaction);

        // Act & Assert
        await Assert.ThrowsAsync<ArgumentException>(
            () => handler.HandleAsync(command, CancellationToken.None));
    }

    [Test]
    public async Task Should_ReturnEntitlements_When_VerifySucceeds()
    {
        // Arrange
        var (verifier, decoder, repository, auth0, settings) = CreateDependencies();
        decoder.Register(DecodedJson, CreateTransaction());

        var profile = UserProfile.Register(UserId);
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new VerifySubscriptionCommandHandler(
            verifier, decoder, repository, auth0, settings);
        var command = new VerifySubscriptionCommand(UserId, SignedTransaction);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Entitlements).Contains("StatusChangeAlerts");
        await Assert.That(result.WatchZoneLimit).IsEqualTo(3);
    }

    [Test]
    public async Task Should_ThrowUserProfileNotFoundException_When_UserNotFound()
    {
        // Arrange
        var (verifier, decoder, repository, auth0, settings) = CreateDependencies();
        decoder.Register(DecodedJson, CreateTransaction());

        // No profile saved
        var handler = new VerifySubscriptionCommandHandler(
            verifier, decoder, repository, auth0, settings);
        var command = new VerifySubscriptionCommand(UserId, SignedTransaction);

        // Act & Assert
        await Assert.ThrowsAsync<UserProfileNotFoundException>(
            () => handler.HandleAsync(command, CancellationToken.None));
    }

    [Test]
    public async Task Should_RestorePersonalTier_When_SingleActiveTransactionSupplied()
    {
        // Arrange
        var (verifier, decoder, repository, auth0, settings) = CreateDependencies();
        verifier.SetPayload("restore.jws.1", DecodedJson);
        decoder.Register(DecodedJson, CreateTransaction(expiresDate: Active));

        var profile = UserProfile.Register(UserId);
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new VerifySubscriptionCommandHandler(
            verifier, decoder, repository, auth0, settings);
        var command = new VerifySubscriptionCommand(UserId, SingleRestoreJws);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Personal);
        await Assert.That(result.SubscriptionExpiry).IsEqualTo(Active);
    }

    [Test]
    public async Task Should_ResolveToHighestActiveTier_When_RestoreHasPersonalAndExpiredPro()
    {
        // Arrange — a Personal transaction that is still active, plus a Pro
        // transaction that has lapsed. Restore must ignore the expired Pro and
        // resolve the user to the active Personal tier.
        var (verifier, decoder, repository, auth0, settings) = CreateDependencies();
        verifier.SetPayload("restore.personal", "{\"which\":\"personal\"}");
        verifier.SetPayload("restore.expired-pro", "{\"which\":\"expired-pro\"}");
        decoder.Register(
            "{\"which\":\"personal\"}",
            CreateTransaction(productId: "uk.co.towncrier.personal.monthly", expiresDate: Active));
        decoder.Register(
            "{\"which\":\"expired-pro\"}",
            CreateTransaction(productId: "uk.co.towncrier.pro.monthly", expiresDate: Expired));

        var profile = UserProfile.Register(UserId);
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new VerifySubscriptionCommandHandler(
            verifier, decoder, repository, auth0, settings);
        var command = new VerifySubscriptionCommand(UserId, PersonalAndExpiredProJws);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Personal);
        await Assert.That(result.SubscriptionExpiry).IsEqualTo(Active);
    }

    [Test]
    public async Task Should_ResolveToProTier_When_RestoreHasActiveProAndActivePersonal()
    {
        // Arrange — two active transactions of different tiers. Restore must
        // resolve to the highest one (Pro).
        var (verifier, decoder, repository, auth0, settings) = CreateDependencies();
        verifier.SetPayload("restore.personal", "{\"which\":\"personal\"}");
        verifier.SetPayload("restore.pro", "{\"which\":\"pro\"}");
        decoder.Register(
            "{\"which\":\"personal\"}",
            CreateTransaction(productId: "uk.co.towncrier.personal.monthly", expiresDate: Active));
        decoder.Register(
            "{\"which\":\"pro\"}",
            CreateTransaction(productId: "uk.co.towncrier.pro.monthly", expiresDate: ActiveLater));

        var profile = UserProfile.Register(UserId);
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new VerifySubscriptionCommandHandler(
            verifier, decoder, repository, auth0, settings);
        var command = new VerifySubscriptionCommand(UserId, PersonalAndProJws);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Pro);
        await Assert.That(result.SubscriptionExpiry).IsEqualTo(ActiveLater);
    }

    [Test]
    public async Task Should_ResolveToFree_When_RestoreHasOnlyExpiredTransactions()
    {
        // Arrange
        var (verifier, decoder, repository, auth0, settings) = CreateDependencies();
        verifier.SetPayload("restore.expired", "{\"which\":\"expired\"}");
        decoder.Register(
            "{\"which\":\"expired\"}",
            CreateTransaction(expiresDate: Expired));

        var profile = UserProfile.Register(UserId);
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new VerifySubscriptionCommandHandler(
            verifier, decoder, repository, auth0, settings);
        var command = new VerifySubscriptionCommand(UserId, ExpiredOnlyJws);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Free);
        await Assert.That(result.SubscriptionExpiry).IsNull();
    }

    [Test]
    public async Task Should_ResolveToFree_When_RestoreHasNoTransactions()
    {
        // Arrange
        var (verifier, decoder, repository, auth0, settings) = CreateDependencies();

        var profile = UserProfile.Register(UserId);
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new VerifySubscriptionCommandHandler(
            verifier, decoder, repository, auth0, settings);
        var command = new VerifySubscriptionCommand(UserId, Array.Empty<string>());

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Free);
    }

    [Test]
    public async Task Should_ThrowAppleJwsVerificationException_When_RestoreContainsTamperedJws()
    {
        // Arrange — one valid transaction, one whose JWS is not registered
        // with the verifier (simulating a tampered signature in the list).
        var (verifier, decoder, repository, auth0, settings) = CreateDependencies();
        verifier.SetPayload("restore.valid", DecodedJson);
        decoder.Register(DecodedJson, CreateTransaction(expiresDate: Active));

        var profile = UserProfile.Register(UserId);
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new VerifySubscriptionCommandHandler(
            verifier, decoder, repository, auth0, settings);
        var command = new VerifySubscriptionCommand(UserId, ValidAndTamperedJws);

        // Act & Assert
        await Assert.ThrowsAsync<AppleJwsVerificationException>(
            () => handler.HandleAsync(command, CancellationToken.None));
    }

    [Test]
    public async Task Should_SyncRestoredTierToAuth0_When_RestoreSucceeds()
    {
        // Arrange
        var (verifier, decoder, repository, auth0, settings) = CreateDependencies();
        verifier.SetPayload("restore.jws.1", DecodedJson);
        decoder.Register(DecodedJson, CreateTransaction(expiresDate: Active));

        var profile = UserProfile.Register(UserId);
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new VerifySubscriptionCommandHandler(
            verifier, decoder, repository, auth0, settings);
        var command = new VerifySubscriptionCommand(UserId, SingleRestoreJws);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(auth0.Updates).HasCount().EqualTo(1);
        await Assert.That(auth0.Updates[0].Tier).IsEqualTo("Personal");
    }

    private static DecodedTransaction CreateTransaction(
        string productId = "uk.co.towncrier.personal.monthly",
        string bundleId = BundleId,
        string environment = "Production",
        DateTimeOffset? expiresDate = null) =>
        new(
            TransactionId: "txn-1",
            OriginalTransactionId: "orig-txn-1",
            ProductId: productId,
            BundleId: bundleId,
            PurchaseDate: new DateTimeOffset(2026, 4, 11, 0, 0, 0, TimeSpan.Zero),
            ExpiresDate: expiresDate ?? Active,
            Environment: environment);

    private static (FakeAppleJwsVerifier Verifier, FakeTransactionDecoder Decoder, FakeUserProfileRepository Repository, FakeAuth0ManagementClient Auth0, AppleSettings Settings) CreateDependencies()
    {
        var verifier = new FakeAppleJwsVerifier();
        verifier.SetPayload(SignedTransaction, DecodedJson);

        var decoder = new FakeTransactionDecoder();

        var repository = new FakeUserProfileRepository();
        var auth0 = new FakeAuth0ManagementClient();

        var settings = new AppleSettings
        {
            BundleId = BundleId,
            Environment = "Production",
        };

        return (verifier, decoder, repository, auth0, settings);
    }
}
