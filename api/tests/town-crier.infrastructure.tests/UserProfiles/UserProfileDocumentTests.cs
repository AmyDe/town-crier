using TownCrier.Domain.UserProfiles;
using TownCrier.Infrastructure.UserProfiles;

namespace TownCrier.Infrastructure.Tests.UserProfiles;

public sealed class UserProfileDocumentTests
{
    [Test]
    public async Task Should_SetUserIdAsId_When_MappedFromDomain()
    {
        // Arrange
        var profile = UserProfile.Register("auth0|user-1");

        // Act
        var document = UserProfileDocument.FromDomain(profile);

        // Assert
        await Assert.That(document.Id).IsEqualTo("auth0|user-1");
    }

    [Test]
    public async Task Should_PreserveBasicFields_When_MappedFromDomain()
    {
        // Arrange
        var profile = UserProfile.Register("auth0|user-1");
        profile.UpdatePreferences(new NotificationPreferences(true, DayOfWeek.Wednesday));
        profile.ActivateSubscription(SubscriptionTier.Pro, new DateTimeOffset(2027, 1, 1, 0, 0, 0, TimeSpan.Zero));
        profile.LinkOriginalTransactionId("txn-abc-123");

        // Act
        var document = UserProfileDocument.FromDomain(profile);

        // Assert
        await Assert.That(document.UserId).IsEqualTo("auth0|user-1");
        await Assert.That(document.PushEnabled).IsTrue();
        await Assert.That(document.DigestDay).IsEqualTo(DayOfWeek.Wednesday);
        await Assert.That(document.Tier).IsEqualTo("Pro");
        await Assert.That(document.SubscriptionExpiry).IsEqualTo(new DateTimeOffset(2027, 1, 1, 0, 0, 0, TimeSpan.Zero));
        await Assert.That(document.OriginalTransactionId).IsEqualTo("txn-abc-123");
    }

    [Test]
    public async Task Should_RoundTripToDomain_When_MappedBackAndForth()
    {
        // Arrange
        var original = UserProfile.Register("auth0|user-1");
        original.UpdatePreferences(new NotificationPreferences(false, DayOfWeek.Friday));
        original.ActivateSubscription(SubscriptionTier.Pro, new DateTimeOffset(2027, 6, 15, 0, 0, 0, TimeSpan.Zero));
        original.LinkOriginalTransactionId("txn-round-trip");
        original.EnterGracePeriod(new DateTimeOffset(2027, 7, 1, 0, 0, 0, TimeSpan.Zero));

        // Act
        var document = UserProfileDocument.FromDomain(original);
        var roundTripped = document.ToDomain();

        // Assert
        await Assert.That(roundTripped.UserId).IsEqualTo(original.UserId);
        await Assert.That(roundTripped.NotificationPreferences.PushEnabled).IsEqualTo(original.NotificationPreferences.PushEnabled);
        await Assert.That(roundTripped.NotificationPreferences.DigestDay).IsEqualTo(original.NotificationPreferences.DigestDay);
        await Assert.That(roundTripped.Tier).IsEqualTo(original.Tier);
        await Assert.That(roundTripped.SubscriptionExpiry).IsEqualTo(original.SubscriptionExpiry);
        await Assert.That(roundTripped.OriginalTransactionId).IsEqualTo(original.OriginalTransactionId);
        await Assert.That(roundTripped.GracePeriodExpiry).IsEqualTo(original.GracePeriodExpiry);
    }

    [Test]
    public async Task Should_PreserveZonePreferences_When_RoundTripped()
    {
        // Arrange
        var original = UserProfile.Register("auth0|user-1");
        original.ActivateSubscription(SubscriptionTier.Pro, new DateTimeOffset(2027, 1, 1, 0, 0, 0, TimeSpan.Zero));
        original.SetZonePreferences("zone-1", new ZoneNotificationPreferences(true, true, false, false));
        original.SetZonePreferences("zone-2", new ZoneNotificationPreferences(false, false, true, true));

        // Act
        var document = UserProfileDocument.FromDomain(original);
        var roundTripped = document.ToDomain();

        // Assert
        var zone1 = roundTripped.GetZonePreferences("zone-1");
        await Assert.That(zone1.NewApplicationPush).IsTrue();
        await Assert.That(zone1.NewApplicationEmail).IsTrue();
        await Assert.That(zone1.DecisionPush).IsFalse();
        await Assert.That(zone1.DecisionEmail).IsFalse();

        var zone2 = roundTripped.GetZonePreferences("zone-2");
        await Assert.That(zone2.NewApplicationPush).IsFalse();
        await Assert.That(zone2.NewApplicationEmail).IsFalse();
        await Assert.That(zone2.DecisionPush).IsTrue();
        await Assert.That(zone2.DecisionEmail).IsTrue();
    }

    [Test]
    public async Task Should_HandleNullOptionalFields_When_MappedFromDomain()
    {
        // Arrange — fresh profile has null subscription expiry, etc.
        var profile = UserProfile.Register("auth0|user-1");

        // Act
        var document = UserProfileDocument.FromDomain(profile);

        // Assert
        await Assert.That(document.Tier).IsEqualTo("Free");
        await Assert.That(document.SubscriptionExpiry).IsNull();
        await Assert.That(document.OriginalTransactionId).IsNull();
        await Assert.That(document.GracePeriodExpiry).IsNull();
        await Assert.That(document.ZonePreferences).IsNotNull();
        await Assert.That(document.ZonePreferences).IsEmpty();
    }

    [Test]
    public async Task Should_PreserveEmail_When_RoundTripped()
    {
        // Arrange
        var original = UserProfile.Register("auth0|user-1", "test@example.com");

        // Act
        var document = UserProfileDocument.FromDomain(original);
        var roundTripped = document.ToDomain();

        // Assert
        await Assert.That(document.Email).IsEqualTo("test@example.com");
        await Assert.That(roundTripped.Email).IsEqualTo("test@example.com");
    }

    [Test]
    public async Task Should_HandleNullEmail_When_RoundTripped()
    {
        // Arrange
        var original = UserProfile.Register("auth0|user-1");

        // Act
        var document = UserProfileDocument.FromDomain(original);
        var roundTripped = document.ToDomain();

        // Assert
        await Assert.That(document.Email).IsNull();
        await Assert.That(roundTripped.Email).IsNull();
    }

    [Test]
    public async Task Should_PreserveSavedDecisionFlags_When_RoundTripped()
    {
        // Arrange — toggle the saved-decision flags off so we can verify they persist.
        var original = UserProfile.Register("auth0|user-1");
        original.UpdatePreferences(new NotificationPreferences(
            PushEnabled: true,
            DigestDay: DayOfWeek.Monday,
            EmailDigestEnabled: true,
            SavedDecisionPush: false,
            SavedDecisionEmail: false));

        // Act
        var document = UserProfileDocument.FromDomain(original);
        var roundTripped = document.ToDomain();

        // Assert
        await Assert.That(roundTripped.NotificationPreferences.SavedDecisionPush).IsFalse();
        await Assert.That(roundTripped.NotificationPreferences.SavedDecisionEmail).IsFalse();
    }

    [Test]
    public async Task Should_DefaultSavedDecisionFlagsToTrue_When_LegacyDocumentLacksFields()
    {
        // Arrange — simulate a legacy Cosmos document predating the saved-decision fields
        // by constructing the document directly with the new fields unset (null).
        var document = new UserProfileDocument
        {
            Id = "auth0|user-legacy",
            UserId = "auth0|user-legacy",
            Email = null,
            PushEnabled = true,
            DigestDay = DayOfWeek.Monday,
            EmailDigestEnabled = true,
            SavedDecisionPush = null,
            SavedDecisionEmail = null,
            ZonePreferences = new Dictionary<string, ZoneNotificationPreferences>(),
            Tier = "Free",
            LastActiveAt = DateTimeOffset.UtcNow,
        };

        // Act
        var profile = document.ToDomain();

        // Assert — legacy documents hydrate with both flags defaulting to true.
        await Assert.That(profile.NotificationPreferences.SavedDecisionPush).IsTrue();
        await Assert.That(profile.NotificationPreferences.SavedDecisionEmail).IsTrue();
    }
}
