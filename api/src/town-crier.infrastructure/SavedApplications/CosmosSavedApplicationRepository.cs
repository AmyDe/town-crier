using System.Net;
using Microsoft.Azure.Cosmos;
using TownCrier.Application.SavedApplications;
using TownCrier.Domain.SavedApplications;

namespace TownCrier.Infrastructure.SavedApplications;

public sealed class CosmosSavedApplicationRepository : ISavedApplicationRepository
{
    private readonly Container container;

    public CosmosSavedApplicationRepository(CosmosClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.container = client.GetContainer("town-crier", "SavedApplications");
    }

    public async Task SaveAsync(SavedApplication savedApplication, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(savedApplication);

        var document = SavedApplicationDocument.FromDomain(savedApplication);
        await this.container.UpsertItemAsync(
            document,
            new PartitionKey(document.UserId),
            cancellationToken: ct).ConfigureAwait(false);
    }

    public async Task DeleteAsync(string userId, string applicationUid, CancellationToken ct)
    {
        var id = SavedApplicationDocument.MakeId(userId, applicationUid);

        try
        {
            await this.container.DeleteItemAsync<SavedApplicationDocument>(
                id,
                new PartitionKey(userId),
                cancellationToken: ct).ConfigureAwait(false);
        }
        catch (CosmosException ex) when (ex.StatusCode == HttpStatusCode.NotFound)
        {
            // Idempotent delete — item already removed
        }
    }

    public async Task<IReadOnlyList<SavedApplication>> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        var query = new QueryDefinition("SELECT * FROM c WHERE c.userId = @userId")
            .WithParameter("@userId", userId);

        using var iterator = this.container.GetItemQueryIterator<SavedApplicationDocument>(
            query,
            requestOptions: new QueryRequestOptions { PartitionKey = new PartitionKey(userId) });

        var results = new List<SavedApplication>();

        while (iterator.HasMoreResults)
        {
            var response = await iterator.ReadNextAsync(ct).ConfigureAwait(false);
            results.AddRange(response.Select(doc => doc.ToDomain()));
        }

        return results;
    }

    public async Task<bool> ExistsAsync(string userId, string applicationUid, CancellationToken ct)
    {
        var id = SavedApplicationDocument.MakeId(userId, applicationUid);

        try
        {
            await this.container.ReadItemAsync<SavedApplicationDocument>(
                id,
                new PartitionKey(userId),
                cancellationToken: ct).ConfigureAwait(false);
            return true;
        }
        catch (CosmosException ex) when (ex.StatusCode == HttpStatusCode.NotFound)
        {
            return false;
        }
    }

    public async Task<IReadOnlyList<string>> GetUserIdsByApplicationUidAsync(string applicationUid, CancellationToken ct)
    {
        var query = new QueryDefinition("SELECT c.userId FROM c WHERE c.applicationUid = @applicationUid")
            .WithParameter("@applicationUid", applicationUid);

        using var iterator = this.container.GetItemQueryIterator<SavedApplicationDocument>(
            query,
            requestOptions: new QueryRequestOptions { PartitionKey = null });

        var userIds = new List<string>();

        while (iterator.HasMoreResults)
        {
            var response = await iterator.ReadNextAsync(ct).ConfigureAwait(false);
            userIds.AddRange(response.Select(doc => doc.UserId));
        }

        return userIds;
    }
}
