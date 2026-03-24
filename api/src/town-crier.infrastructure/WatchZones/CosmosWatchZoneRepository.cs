using System.Net;
using Microsoft.Azure.Cosmos;
using TownCrier.Application.WatchZones;
using TownCrier.Domain.WatchZones;

namespace TownCrier.Infrastructure.WatchZones;

public sealed class CosmosWatchZoneRepository : IWatchZoneRepository
{
    private readonly Container container;

    public CosmosWatchZoneRepository(CosmosClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.container = client.GetContainer("town-crier", "WatchZones");
    }

    public async Task SaveAsync(WatchZone zone, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(zone);

        var document = WatchZoneDocument.FromDomain(zone);
        await this.container.UpsertItemAsync(
            document,
            new PartitionKey(document.UserId),
            cancellationToken: ct).ConfigureAwait(false);
    }

    public async Task<IReadOnlyCollection<WatchZone>> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);

        var query = new QueryDefinition("SELECT * FROM c WHERE c.userId = @userId")
            .WithParameter("@userId", userId);

        using var iterator = this.container.GetItemQueryIterator<WatchZoneDocument>(
            query,
            requestOptions: new QueryRequestOptions { PartitionKey = new PartitionKey(userId) });

        var results = new List<WatchZone>();

        while (iterator.HasMoreResults)
        {
            var response = await iterator.ReadNextAsync(ct).ConfigureAwait(false);
            results.AddRange(response.Select(doc => doc.ToDomain()));
        }

        return results;
    }

    public async Task DeleteAsync(string userId, string zoneId, CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);
        ArgumentException.ThrowIfNullOrWhiteSpace(zoneId);

        try
        {
            await this.container.DeleteItemAsync<WatchZoneDocument>(
                zoneId,
                new PartitionKey(userId),
                cancellationToken: ct).ConfigureAwait(false);
        }
        catch (CosmosException ex) when (ex.StatusCode == HttpStatusCode.NotFound)
        {
            throw new WatchZoneNotFoundException();
        }
    }

    public async Task<IReadOnlyCollection<WatchZone>> FindZonesContainingAsync(
        double latitude, double longitude, CancellationToken ct)
    {
        var query = new QueryDefinition(
            "SELECT * FROM c WHERE ST_DISTANCE({'type': 'Point', 'coordinates': [c.longitude, c.latitude]}, " +
            "{'type': 'Point', 'coordinates': [@longitude, @latitude]}) <= c.radiusMetres")
            .WithParameter("@latitude", latitude)
            .WithParameter("@longitude", longitude);

        using var iterator = this.container.GetItemQueryIterator<WatchZoneDocument>(
            query,
            requestOptions: new QueryRequestOptions { PartitionKey = null });

        var results = new List<WatchZone>();

        while (iterator.HasMoreResults)
        {
            var response = await iterator.ReadNextAsync(ct).ConfigureAwait(false);
            results.AddRange(response.Select(doc => doc.ToDomain()));
        }

        return results;
    }

    public async Task<IReadOnlyCollection<int>> GetDistinctAuthorityIdsAsync(CancellationToken ct)
    {
        var query = new QueryDefinition("SELECT DISTINCT VALUE c.authorityId FROM c");

        using var iterator = this.container.GetItemQueryIterator<int>(
            query,
            requestOptions: new QueryRequestOptions { PartitionKey = null });

        var results = new List<int>();

        while (iterator.HasMoreResults)
        {
            var response = await iterator.ReadNextAsync(ct).ConfigureAwait(false);
            results.AddRange(response);
        }

        return results;
    }

    public async Task<Dictionary<int, int>> GetZoneCountsByAuthorityAsync(CancellationToken ct)
    {
        var query = new QueryDefinition(
            "SELECT c.authorityId, COUNT(1) AS zoneCount FROM c GROUP BY c.authorityId");

        using var iterator = this.container.GetItemQueryIterator<AuthorityZoneCountResult>(
            query,
            requestOptions: new QueryRequestOptions { PartitionKey = null });

        var results = new Dictionary<int, int>();

        while (iterator.HasMoreResults)
        {
            var response = await iterator.ReadNextAsync(ct).ConfigureAwait(false);

            foreach (var item in response)
            {
                results[item.AuthorityId] = item.ZoneCount;
            }
        }

        return results;
    }
}
