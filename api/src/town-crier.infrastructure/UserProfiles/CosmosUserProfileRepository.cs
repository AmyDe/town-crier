using System.Net;
using Microsoft.Azure.Cosmos;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.UserProfiles;

public sealed class CosmosUserProfileRepository : IUserProfileRepository
{
    private readonly Container container;

    public CosmosUserProfileRepository(CosmosClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.container = client.GetContainer(CosmosContainerNames.DatabaseName, CosmosContainerNames.Users);
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

        using var iterator = this.container.GetItemQueryIterator<UserProfileDocument>(queryDefinition);

        return await iterator.CollectAsync(doc => doc.ToDomain(), ct).ConfigureAwait(false);
    }

    public async Task<UserProfile?> GetByOriginalTransactionIdAsync(
        string originalTransactionId,
        CancellationToken ct)
    {
        var queryDefinition = new QueryDefinition(
            "SELECT * FROM c WHERE c.originalTransactionId = @txnId")
            .WithParameter("@txnId", originalTransactionId);

        using var iterator = this.container.GetItemQueryIterator<UserProfileDocument>(queryDefinition);

        return await iterator.FirstOrDefaultAsync(doc => doc.ToDomain(), ct).ConfigureAwait(false);
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
