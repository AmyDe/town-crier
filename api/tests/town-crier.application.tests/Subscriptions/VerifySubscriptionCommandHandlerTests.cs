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

    private static DecodedTransaction CreateTransaction(
        string productId = "uk.co.towncrier.personal.monthly",
        string bundleId = BundleId,
        string environment = "Production") =>
        new(
            TransactionId: "txn-1",
            OriginalTransactionId: "orig-txn-1",
            ProductId: productId,
            BundleId: bundleId,
            PurchaseDate: new DateTimeOffset(2026, 4, 11, 0, 0, 0, TimeSpan.Zero),
            ExpiresDate: new DateTimeOffset(2026, 5, 11, 0, 0, 0, TimeSpan.Zero),
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
