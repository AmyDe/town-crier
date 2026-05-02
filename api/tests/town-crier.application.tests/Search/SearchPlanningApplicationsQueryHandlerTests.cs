using TownCrier.Application.Search;
using TownCrier.Application.Tests.Notifications;
using TownCrier.Application.Tests.Polling;
using TownCrier.Application.Tests.UserProfiles;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.Search;

public sealed class SearchPlanningApplicationsQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnResults_When_UserIsFreeTier()
    {
        // Arrange — tier check is now in the endpoint filter, not the handler
        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Free)
            .Build();
        var userProfileRepository = new FakeUserProfileRepository();
        await userProfileRepository.SaveAsync(profile, CancellationToken.None);

        var planItClient = new FakePlanItClient();
        planItClient.SearchTotal = 0;
        var appRepo = new FakePlanningApplicationRepository();
        var handler = new SearchPlanningApplicationsQueryHandler(userProfileRepository, planItClient, appRepo);

        var query = new SearchPlanningApplicationsQuery("user-1", "extension", AuthorityId: 42);

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert — handler no longer rejects free tier
        await Assert.That(result.Applications).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_ReturnSearchResults_When_UserIsProTier()
    {
        // Arrange
        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Pro)
            .Build();
        var userProfileRepository = new FakeUserProfileRepository();
        await userProfileRepository.SaveAsync(profile, CancellationToken.None);

        var application = new PlanningApplicationBuilder()
            .WithName("Extension to rear")
            .WithUid("planit-123")
            .Build();
        var planItClient = new FakePlanItClient();
        planItClient.AddSearchResult(application);
        planItClient.SearchTotal = 1;

        var appRepo = new FakePlanningApplicationRepository();
        var handler = new SearchPlanningApplicationsQueryHandler(userProfileRepository, planItClient, appRepo);
        var query = new SearchPlanningApplicationsQuery("user-1", "extension", AuthorityId: 42);

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result.Applications).HasCount().EqualTo(1);
        await Assert.That(result.Applications.First().Uid).IsEqualTo("planit-123");
        await Assert.That(result.Applications.First().Name).IsEqualTo("Extension to rear");
        await Assert.That(result.Total).IsEqualTo(1);
        await Assert.That(result.Page).IsEqualTo(1);
        await Assert.That(planItClient.LastSearchText).IsEqualTo("extension");
        await Assert.That(planItClient.LastAuthorityId).IsEqualTo(42);
    }

    [Test]
    public async Task Should_NotUpsertSearchResults_Into_Repository()
    {
        // Arrange. Search must not upsert results into Cosmos. The previous
        // per-application upsert was the dominant source of the 429 burst on
        // user-facing requests. Apps are upserted lazily on save (see
        // SaveApplicationCommandHandler) and on detail-page Cosmos miss (see
        // GetApplicationByUidQueryHandler). Bead tc-if12.
        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Pro)
            .Build();
        var userProfileRepository = new FakeUserProfileRepository();
        await userProfileRepository.SaveAsync(profile, CancellationToken.None);

        var application = new PlanningApplicationBuilder()
            .WithName("Extension to rear")
            .WithUid("planit-123")
            .Build();
        var planItClient = new FakePlanItClient();
        planItClient.AddSearchResult(application);
        planItClient.SearchTotal = 1;

        var appRepo = new FakePlanningApplicationRepository();
        var handler = new SearchPlanningApplicationsQueryHandler(userProfileRepository, planItClient, appRepo);
        var query = new SearchPlanningApplicationsQuery("user-1", "extension", AuthorityId: 42);

        // Act
        await handler.HandleAsync(query, CancellationToken.None);

        // Assert. Zero Cosmos writes per search — the acceptance criterion.
        await Assert.That(appRepo.UpsertCallCount).IsEqualTo(0);
        var stored = await appRepo.GetByUidAsync("planit-123", CancellationToken.None);
        await Assert.That(stored).IsNull();
    }

    [Test]
    public async Task Should_ReturnEmptyResults_When_NoMatchesFound()
    {
        // Arrange
        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Pro)
            .Build();
        var userProfileRepository = new FakeUserProfileRepository();
        await userProfileRepository.SaveAsync(profile, CancellationToken.None);

        var planItClient = new FakePlanItClient();
        planItClient.SearchTotal = 0;

        var appRepo = new FakePlanningApplicationRepository();
        var handler = new SearchPlanningApplicationsQueryHandler(userProfileRepository, planItClient, appRepo);
        var query = new SearchPlanningApplicationsQuery("user-1", "nonexistent", AuthorityId: 42);

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result.Applications).HasCount().EqualTo(0);
        await Assert.That(result.Total).IsEqualTo(0);
    }

    [Test]
    public async Task Should_ThrowUserProfileNotFound_When_UserDoesNotExist()
    {
        // Arrange
        var userProfileRepository = new FakeUserProfileRepository();
        var planItClient = new FakePlanItClient();
        var appRepo = new FakePlanningApplicationRepository();
        var handler = new SearchPlanningApplicationsQueryHandler(userProfileRepository, planItClient, appRepo);
        var query = new SearchPlanningApplicationsQuery("nonexistent-user", "extension", AuthorityId: 42);

        // Act & Assert
        await Assert.ThrowsAsync<UserProfileNotFoundException>(
            () => handler.HandleAsync(query, CancellationToken.None));
    }
}
