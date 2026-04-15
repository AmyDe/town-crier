using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.UserProfiles;

public sealed class CosmosUserProfileRepository : IUserProfileRepository
{
    private readonly ICosmosRestClient client;

    public CosmosUserProfileRepository(ICosmosRestClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.client = client;
    }

    public async Task<UserProfile?> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        var document = await this.client.ReadDocumentAsync(
            CosmosContainerNames.Users,
            userId,
            userId,
            CosmosJsonSerializerContext.Default.UserProfileDocument,
            ct).ConfigureAwait(false);

        return document?.ToDomain();
    }

    public async Task<UserProfile?> GetByEmailAsync(string email, CancellationToken ct)
    {
        var documents = await this.client.QueryAsync(
            CosmosContainerNames.Users,
            "SELECT * FROM c WHERE c.email = @email",
            [new QueryParameter("@email", email)],
            partitionKey: null,
            CosmosJsonSerializerContext.Default.UserProfileDocument,
            ct).ConfigureAwait(false);

        return documents.Count > 0 ? documents[0].ToDomain() : null;
    }

    public async Task<IReadOnlyList<UserProfile>> GetAllByTierAsync(SubscriptionTier tier, CancellationToken ct)
    {
        var documents = await this.client.QueryAsync(
            CosmosContainerNames.Users,
            "SELECT * FROM c WHERE c.tier = @tier",
            [new QueryParameter("@tier", tier.ToString())],
            partitionKey: null,
            CosmosJsonSerializerContext.Default.UserProfileDocument,
            ct).ConfigureAwait(false);

        return documents.ConvertAll(doc => doc.ToDomain());
    }

    public async Task<IReadOnlyList<UserProfile>> GetAllByDigestDayAsync(DayOfWeek digestDay, CancellationToken ct)
    {
        var documents = await this.client.QueryAsync(
            CosmosContainerNames.Users,
            "SELECT * FROM c WHERE c.digestDay = @digestDay",
            [new QueryParameter("@digestDay", (int)digestDay)],
            partitionKey: null,
            CosmosJsonSerializerContext.Default.UserProfileDocument,
            ct).ConfigureAwait(false);

        return documents.ConvertAll(doc => doc.ToDomain());
    }

    public async Task<UserProfile?> GetByOriginalTransactionIdAsync(
        string originalTransactionId,
        CancellationToken ct)
    {
        var documents = await this.client.QueryAsync(
            CosmosContainerNames.Users,
            "SELECT * FROM c WHERE c.originalTransactionId = @txnId",
            [new QueryParameter("@txnId", originalTransactionId)],
            partitionKey: null,
            CosmosJsonSerializerContext.Default.UserProfileDocument,
            ct).ConfigureAwait(false);

        return documents.Count > 0 ? documents[0].ToDomain() : null;
    }

    public async Task<UserProfilePage> ListAsync(
        string? emailSearch,
        int pageSize,
        string? continuationToken,
        CancellationToken ct)
    {
        var sql = emailSearch is not null
            ? "SELECT * FROM c WHERE CONTAINS(c.email, @search, true) ORDER BY c.email"
            : "SELECT * FROM c ORDER BY c.email";

        var parameters = emailSearch is not null
            ? new[] { new QueryParameter("@search", emailSearch) }
            : Array.Empty<QueryParameter>();

        var result = await this.client.QueryPageAsync(
            CosmosContainerNames.Users,
            sql,
            parameters,
            partitionKey: null,
            pageSize,
            continuationToken,
            CosmosJsonSerializerContext.Default.UserProfileDocument,
            ct).ConfigureAwait(false);

        var profiles = result.Items.Select(doc => doc.ToDomain()).ToList();
        return new UserProfilePage(profiles, result.ContinuationToken);
    }

    public async Task SaveAsync(UserProfile profile, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(profile);

        var document = UserProfileDocument.FromDomain(profile);
        await this.client.UpsertDocumentAsync(
            CosmosContainerNames.Users,
            document,
            document.Id,
            CosmosJsonSerializerContext.Default.UserProfileDocument,
            ct).ConfigureAwait(false);
    }

    public async Task DeleteAsync(string userId, CancellationToken ct)
    {
        await this.client.DeleteDocumentAsync(
            CosmosContainerNames.Users,
            userId,
            userId,
            ct).ConfigureAwait(false);
    }
}
