using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.Tests.Polling;
using TownCrier.Application.Tests.UserProfiles;
using TownCrier.Application.WatchZones;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.WatchZones;

public sealed class CreateWatchZoneCommandHandlerTests
{
    private static readonly DateTimeOffset FixedNow = new(2026, 3, 17, 12, 0, 0, TimeSpan.Zero);

    private readonly FakeAuthorityResolver authorityResolver = new();
    private readonly FakePlanItClient planItClient = new();
    private readonly FakePlanningApplicationRepository planningApplicationRepository = new();
    private readonly FakeUserProfileRepository userProfileRepository = new();
    private readonly FakeWatchZoneRepository watchZoneRepository = new();

    [Test]
    public async Task Should_SaveWatchZone_When_FreeUserCreatesZone()
    {
        // Arrange
        var profile = UserProfile.Register("user-1");
        await this.userProfileRepository.SaveAsync(profile, CancellationToken.None);

        var handler = this.CreateHandler();
        var command = CreateCommand();

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var zones = await this.watchZoneRepository.FindZonesContainingAsync(51.5074, -0.1278, CancellationToken.None);
        await Assert.That(zones).HasCount().EqualTo(1);
        await Assert.That(zones.First().UserId).IsEqualTo("user-1");
        await Assert.That(zones.First().AuthorityId).IsEqualTo(42);
        await Assert.That(zones.First().Name).IsEqualTo("My Zone");
    }

    [Test]
    public async Task Should_NotCallPlanIt_When_FreeUserCreatesZone()
    {
        // Arrange
        var profile = UserProfile.Register("user-1");
        await this.userProfileRepository.SaveAsync(profile, CancellationToken.None);

        var handler = this.CreateHandler();
        var command = CreateCommand();

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(this.planItClient.AuthorityIdsRequested).HasCount().EqualTo(0);
        await Assert.That(this.planningApplicationRepository.GetAll()).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_BackfillApplications_When_ProUserCreatesZone()
    {
        // Arrange
        var profile = UserProfile.Register("user-1");
        profile.ActivateSubscription(SubscriptionTier.Pro, new DateTimeOffset(2027, 1, 1, 0, 0, 0, TimeSpan.Zero));
        await this.userProfileRepository.SaveAsync(profile, CancellationToken.None);

        var app1 = new PlanningApplicationBuilder()
            .WithUid("app-1")
            .WithName("App One")
            .WithAreaId(42)
            .WithCoordinates(51.5074, -0.1278)
            .Build();
        var app2 = new PlanningApplicationBuilder()
            .WithUid("app-2")
            .WithName("App Two")
            .WithAreaId(42)
            .WithCoordinates(51.5080, -0.1280)
            .Build();
        this.planItClient.Add(42, app1);
        this.planItClient.Add(42, app2);

        var handler = this.CreateHandler();
        var command = CreateCommand();

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert — backfilled apps are upserted and returned as nearby
        await Assert.That(this.planItClient.AuthorityIdsRequested).Contains(42);
        await Assert.That(this.planningApplicationRepository.GetAll()).HasCount().EqualTo(2);
        await Assert.That(result.NearbyApplications).HasCount().EqualTo(2);
    }

    [Test]
    public async Task Should_UseNinetyDayBackfillWindow_When_ProUserCreatesZone()
    {
        // Arrange
        var profile = UserProfile.Register("user-1");
        profile.ActivateSubscription(SubscriptionTier.Pro, new DateTimeOffset(2027, 1, 1, 0, 0, 0, TimeSpan.Zero));
        await this.userProfileRepository.SaveAsync(profile, CancellationToken.None);

        var handler = this.CreateHandler();
        var command = CreateCommand();

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert — differentStart should be 90 days before 2026-03-17T12:00:00Z
        var expected = new DateTimeOffset(2025, 12, 17, 12, 0, 0, TimeSpan.Zero);
        await Assert.That(this.planItClient.LastDifferentStartUsed).IsEqualTo(expected);
    }

    [Test]
    public async Task Should_SaveWatchZone_When_ProUserCreatesZone()
    {
        // Arrange
        var profile = UserProfile.Register("user-1");
        profile.ActivateSubscription(SubscriptionTier.Pro, new DateTimeOffset(2027, 1, 1, 0, 0, 0, TimeSpan.Zero));
        await this.userProfileRepository.SaveAsync(profile, CancellationToken.None);

        var handler = this.CreateHandler();
        var command = CreateCommand();

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var zones = await this.watchZoneRepository.FindZonesContainingAsync(51.5074, -0.1278, CancellationToken.None);
        await Assert.That(zones).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_ReturnCachedNearbyApplications_When_FreeUserCreatesZone()
    {
        // Arrange
        var profile = UserProfile.Register("user-1");
        await this.userProfileRepository.SaveAsync(profile, CancellationToken.None);

        var nearbyApp = new PlanningApplicationBuilder()
            .WithName("nearby-app")
            .WithAreaId(42)
            .WithCoordinates(51.5080, -0.1280)
            .Build();
        await this.planningApplicationRepository.UpsertAsync(nearbyApp, CancellationToken.None);

        var handler = this.CreateHandler();
        var command = CreateCommand();

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert — free users see cached applications within radius
        await Assert.That(result.NearbyApplications).HasCount().EqualTo(1);
        await Assert.That(result.NearbyApplications.First().Name).IsEqualTo("nearby-app");
    }

    [Test]
    public async Task Should_ReturnEmptyGracefully_When_NoCachedApplicationsExist()
    {
        // Arrange
        var profile = UserProfile.Register("user-1");
        await this.userProfileRepository.SaveAsync(profile, CancellationToken.None);

        var handler = this.CreateHandler();
        var command = CreateCommand();

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.NearbyApplications).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_ExcludeDistantApplications_When_FreeUserCreatesZone()
    {
        // Arrange
        var profile = UserProfile.Register("user-1");
        await this.userProfileRepository.SaveAsync(profile, CancellationToken.None);

        var distantApp = new PlanningApplicationBuilder()
            .WithName("distant-app")
            .WithCoordinates(50.8225, -0.1372) // Brighton — ~76km from London
            .Build();
        await this.planningApplicationRepository.UpsertAsync(distantApp, CancellationToken.None);

        var handler = this.CreateHandler();
        var command = CreateCommand();

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.NearbyApplications).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_ExcludeApplicationsWithoutCoordinates_When_FreeUserCreatesZone()
    {
        // Arrange
        var profile = UserProfile.Register("user-1");
        await this.userProfileRepository.SaveAsync(profile, CancellationToken.None);

        var noCoordApp = new PlanningApplicationBuilder()
            .WithName("no-coord-app")
            .Build();
        await this.planningApplicationRepository.UpsertAsync(noCoordApp, CancellationToken.None);

        var handler = this.CreateHandler();
        var command = CreateCommand();

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.NearbyApplications).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_ReturnOnlyMatchingAuthorityApplications_When_CreatingWatchZone()
    {
        // Arrange
        var profile = UserProfile.Register("user-1");
        await this.userProfileRepository.SaveAsync(profile, CancellationToken.None);

        var matchingApp = new PlanningApplicationBuilder()
            .WithName("matching-app")
            .WithAreaId(42)
            .WithCoordinates(51.5074, -0.1278)
            .Build();
        var differentAuthorityApp = new PlanningApplicationBuilder()
            .WithName("other-authority-app")
            .WithAreaId(99)
            .WithCoordinates(51.5074, -0.1278)
            .Build();
        await this.planningApplicationRepository.UpsertAsync(matchingApp, CancellationToken.None);
        await this.planningApplicationRepository.UpsertAsync(differentAuthorityApp, CancellationToken.None);

        var handler = this.CreateHandler();
        var command = CreateCommand();

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert — only the matching authority's app should be returned
        await Assert.That(result.NearbyApplications).HasCount().EqualTo(1);
        await Assert.That(result.NearbyApplications.First().Name).IsEqualTo("matching-app");
    }

    [Test]
    public async Task Should_ThrowInvalidOperation_When_UserProfileNotFound()
    {
        // Arrange
        var handler = this.CreateHandler();
        var command = new CreateWatchZoneCommand(
            UserId: "nonexistent-user",
            ZoneId: "zone-1",
            Name: "My Zone",
            Latitude: 51.5074,
            Longitude: -0.1278,
            RadiusMetres: 5000,
            AuthorityId: 42);

        // Act & Assert
        await Assert.ThrowsAsync<InvalidOperationException>(
            () => handler.HandleAsync(command, CancellationToken.None));
    }

    [Test]
    public async Task Should_ResolveAuthorityFromCoordinates_When_AuthorityIdNotProvided()
    {
        // Arrange
        var profile = UserProfile.Register("user-1");
        await this.userProfileRepository.SaveAsync(profile, CancellationToken.None);

        this.authorityResolver.Add(51.5074, -0.1278, 42);

        var handler = this.CreateHandler();
        var command = new CreateWatchZoneCommand(
            UserId: "user-1",
            ZoneId: "zone-1",
            Name: "My Zone",
            Latitude: 51.5074,
            Longitude: -0.1278,
            RadiusMetres: 5000);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var zones = await this.watchZoneRepository.FindZonesContainingAsync(51.5074, -0.1278, CancellationToken.None);
        await Assert.That(zones).HasCount().EqualTo(1);
        await Assert.That(zones.First().AuthorityId).IsEqualTo(42);
    }

    [Test]
    public async Task Should_NotCallAuthorityResolver_When_AuthorityIdProvided()
    {
        // Arrange
        var profile = UserProfile.Register("user-1");
        await this.userProfileRepository.SaveAsync(profile, CancellationToken.None);

        var handler = this.CreateHandler();
        var command = CreateCommand();

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(this.authorityResolver.CallCount).IsEqualTo(0);
    }

    [Test]
    public async Task Should_ThrowInvalidOperation_When_AuthorityCannotBeResolved()
    {
        // Arrange
        var profile = UserProfile.Register("user-1");
        await this.userProfileRepository.SaveAsync(profile, CancellationToken.None);

        // No authority resolver mapping configured — resolution will fail
        var handler = this.CreateHandler();
        var command = new CreateWatchZoneCommand(
            UserId: "user-1",
            ZoneId: "zone-1",
            Name: "My Zone",
            Latitude: 99.0,
            Longitude: 99.0,
            RadiusMetres: 5000);

        // Act & Assert
        await Assert.ThrowsAsync<InvalidOperationException>(
            () => handler.HandleAsync(command, CancellationToken.None));
    }

    private static CreateWatchZoneCommand CreateCommand(string userId = "user-1")
    {
        return new CreateWatchZoneCommand(
            UserId: userId,
            ZoneId: "zone-1",
            Name: "My Zone",
            Latitude: 51.5074,
            Longitude: -0.1278,
            RadiusMetres: 5000,
            AuthorityId: 42);
    }

    private CreateWatchZoneCommandHandler CreateHandler()
    {
        return new CreateWatchZoneCommandHandler(
            this.watchZoneRepository,
            this.userProfileRepository,
            this.planItClient,
            this.planningApplicationRepository,
            this.authorityResolver,
            new FakeTimeProvider(FixedNow),
            NullLogger<CreateWatchZoneCommandHandler>.Instance);
    }
}
