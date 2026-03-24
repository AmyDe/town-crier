using System.Net;
using Microsoft.Azure.Cosmos;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Infrastructure.UserProfiles;

public sealed class CosmosUserProfileRepository : IUserProfileRepository
{
    private readonly Container container;

    public CosmosUserProfileRepository(CosmosClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.container = client.GetContainer("town-crier", "Users");
    }

    public async Task<UserProfile?> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        try
        {
            var response = await this.container.ReadItemAsync<UserProfileDocument>(
                userId,
                new PartitionKey(userId),
                cancellationToken: ct).ConfigureAwait(false);

            return response.Resource.ToDomain();
        }
        catch (CosmosException ex) when (ex.StatusCode == HttpStatusCode.NotFound)
        {
            return null;
        }
    }

    public async Task<IReadOnlyList<UserProfile>> GetAllByTierAsync(SubscriptionTier tier, CancellationToken ct)
    {
        var queryDefinition = new QueryDefinition("SELECT * FROM c WHERE c.tier = @tier")
            .WithParameter("@tier", tier.ToString());

        var results = new List<UserProfile>();

        using var iterator = this.container.GetItemQueryIterator<UserProfileDocument>(queryDefinition);
        while (iterator.HasMoreResults)
        {
            var response = await iterator.ReadNextAsync(ct).ConfigureAwait(false);
            results.AddRange(response.Select(doc => doc.ToDomain()));
        }

        return results;
    }

    public async Task<UserProfile?> GetByOriginalTransactionIdAsync(
        string originalTransactionId,
        CancellationToken ct)
    {
        var queryDefinition = new QueryDefinition(
            "SELECT * FROM c WHERE c.originalTransactionId = @txnId")
            .WithParameter("@txnId", originalTransactionId);

        using var iterator = this.container.GetItemQueryIterator<UserProfileDocument>(queryDefinition);
        while (iterator.HasMoreResults)
        {
            var response = await iterator.ReadNextAsync(ct).ConfigureAwait(false);
            var document = response.FirstOrDefault();
            if (document is not null)
            {
                return document.ToDomain();
            }
        }

        return null;
    }

    public async Task SaveAsync(UserProfile profile, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(profile);

        var document = UserProfileDocument.FromDomain(profile);
        await this.container.UpsertItemAsync(
            document,
            new PartitionKey(document.Id),
            cancellationToken: ct).ConfigureAwait(false);
    }

    public async Task DeleteAsync(string userId, CancellationToken ct)
    {
        try
        {
            await this.container.DeleteItemAsync<UserProfileDocument>(
                userId,
                new PartitionKey(userId),
                cancellationToken: ct).ConfigureAwait(false);
        }
        catch (CosmosException ex) when (ex.StatusCode == HttpStatusCode.NotFound)
        {
            // Idempotent delete — already gone
        }
    }
}
