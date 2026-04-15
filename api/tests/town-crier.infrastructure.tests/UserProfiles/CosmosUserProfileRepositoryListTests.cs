using TownCrier.Infrastructure.Cosmos;
using TownCrier.Infrastructure.Tests.Cosmos;
using TownCrier.Infrastructure.UserProfiles;

namespace TownCrier.Infrastructure.Tests.UserProfiles;

public sealed class CosmosUserProfileRepositoryListTests
{
    [Test]
    public async Task Should_ReturnProfiles_When_NoSearchTerm()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosUserProfileRepository(client);

        var docs = new List<UserProfileDocument>
        {
            CreateDocument("user-1", "alice@example.com", "Pro"),
            CreateDocument("user-2", "bob@example.com", "Free"),
        };
        client.SetPageQueryResults("SELECT * FROM c ORDER BY", docs);

        // Act
        var result = await repo.ListAsync(null, 20, null, CancellationToken.None);

        // Assert
        await Assert.That(result.Profiles).HasCount().EqualTo(2);
        await Assert.That(result.Profiles[0].UserId).IsEqualTo("user-1");
        await Assert.That(result.Profiles[0].Email).IsEqualTo("alice@example.com");
    }

    [Test]
    public async Task Should_ReturnProfiles_When_SearchTermProvided()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosUserProfileRepository(client);

        var docs = new List<UserProfileDocument>
        {
            CreateDocument("user-1", "alice@gmail.com", "Personal"),
        };
        client.SetPageQueryResults("SELECT * FROM c WHERE CONTAINS", docs);

        // Act
        var result = await repo.ListAsync("gmail", 20, null, CancellationToken.None);

        // Assert
        await Assert.That(result.Profiles).HasCount().EqualTo(1);
        await Assert.That(result.Profiles[0].Email).IsEqualTo("alice@gmail.com");
    }

    [Test]
    public async Task Should_ForwardContinuationToken_When_MorePagesExist()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosUserProfileRepository(client);

        var docs = new List<UserProfileDocument>
        {
            CreateDocument("user-1", "alice@example.com", "Free"),
        };
        client.SetPageQueryResults("SELECT * FROM c ORDER BY", docs, "next-page-token");

        // Act
        var result = await repo.ListAsync(null, 1, null, CancellationToken.None);

        // Assert
        await Assert.That(result.ContinuationToken).IsEqualTo("next-page-token");
    }

    private static UserProfileDocument CreateDocument(string userId, string email, string tier)
    {
        return new UserProfileDocument
        {
            Id = userId,
            UserId = userId,
            Email = email,
            PushEnabled = true,
            DigestDay = DayOfWeek.Monday,
            EmailDigestEnabled = true,
            ZonePreferences = new Dictionary<string, TownCrier.Domain.UserProfiles.ZoneNotificationPreferences>(),
            Tier = tier,
        };
    }
}
