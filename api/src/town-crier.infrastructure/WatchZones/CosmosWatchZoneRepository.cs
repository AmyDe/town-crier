using TownCrier.Application.WatchZones;
using TownCrier.Domain.WatchZones;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.WatchZones;

public sealed class CosmosWatchZoneRepository : IWatchZoneRepository
{
    private readonly ICosmosRestClient client;

    public CosmosWatchZoneRepository(ICosmosRestClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.client = client;
    }

    public async Task SaveAsync(WatchZone zone, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(zone);

        var document = WatchZoneDocument.FromDomain(zone);
        await this.client.UpsertDocumentAsync(
            CosmosContainerNames.WatchZones,
            document,
            document.UserId,
            CosmosJsonSerializerContext.Default.WatchZoneDocument,
            ct).ConfigureAwait(false);
    }

    public async Task<IReadOnlyCollection<WatchZone>> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);

        var documents = await this.client.QueryAsync(
            CosmosContainerNames.WatchZones,
            "SELECT * FROM c WHERE c.userId = @userId",
            [new QueryParameter("@userId", userId)],
            userId,
            CosmosJsonSerializerContext.Default.WatchZoneDocument,
            ct).ConfigureAwait(false);

        return documents.ConvertAll(doc => doc.ToDomain());
    }

    public async Task DeleteAsync(string userId, string zoneId, CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);
        ArgumentException.ThrowIfNullOrWhiteSpace(zoneId);

        // Read first to preserve WatchZoneNotFoundException semantics,
        // since REST DeleteDocumentAsync is idempotent (no 404 error).
        var existing = await this.client.ReadDocumentAsync(
            CosmosContainerNames.WatchZones,
            zoneId,
            userId,
            CosmosJsonSerializerContext.Default.WatchZoneDocument,
            ct).ConfigureAwait(false);

        if (existing is null)
        {
            throw new WatchZoneNotFoundException();
        }

        await this.client.DeleteDocumentAsync(
            CosmosContainerNames.WatchZones,
            zoneId,
            userId,
            ct).ConfigureAwait(false);
    }

    public async Task<IReadOnlyCollection<WatchZone>> FindZonesContainingAsync(
        double latitude, double longitude, CancellationToken ct)
    {
        const string sql =
            "SELECT * FROM c WHERE ST_DISTANCE({'type': 'Point', 'coordinates': [c.longitude, c.latitude]}, " +
            "{'type': 'Point', 'coordinates': [@longitude, @latitude]}) <= c.radiusMetres";

        var documents = await this.client.QueryAsync(
            CosmosContainerNames.WatchZones,
            sql,
            [new QueryParameter("@latitude", latitude), new QueryParameter("@longitude", longitude)],
            partitionKey: null,
            CosmosJsonSerializerContext.Default.WatchZoneDocument,
            ct).ConfigureAwait(false);

        return documents.ConvertAll(doc => doc.ToDomain());
    }

    public async Task<IReadOnlyCollection<int>> GetDistinctAuthorityIdsAsync(CancellationToken ct)
    {
        return await this.client.QueryAsync(
            CosmosContainerNames.WatchZones,
            "SELECT DISTINCT VALUE c.authorityId FROM c",
            parameters: null,
            partitionKey: null,
            CosmosJsonSerializerContext.Default.Int32,
            ct).ConfigureAwait(false);
    }

    public async Task<Dictionary<int, int>> GetZoneCountsByAuthorityAsync(CancellationToken ct)
    {
        var items = await this.client.QueryAsync(
            CosmosContainerNames.WatchZones,
            "SELECT c.authorityId, COUNT(1) AS zoneCount FROM c GROUP BY c.authorityId",
            parameters: null,
            partitionKey: null,
            CosmosJsonSerializerContext.Default.AuthorityZoneCountResult,
            ct).ConfigureAwait(false);

        return items.ToDictionary(item => item.AuthorityId, item => item.ZoneCount);
    }
}
