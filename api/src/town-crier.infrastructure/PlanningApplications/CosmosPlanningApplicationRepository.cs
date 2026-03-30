using TownCrier.Application.PlanningApplications;
using TownCrier.Domain.PlanningApplications;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.PlanningApplications;

public sealed class CosmosPlanningApplicationRepository : IPlanningApplicationRepository
{
    private readonly ICosmosRestClient client;

    public CosmosPlanningApplicationRepository(ICosmosRestClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.client = client;
    }

    public async Task UpsertAsync(PlanningApplication application, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(application);

        var document = PlanningApplicationDocument.FromDomain(application);
        await this.client.UpsertDocumentAsync(
            CosmosContainerNames.Applications,
            document,
            document.AuthorityCode,
            CosmosJsonSerializerContext.Default.PlanningApplicationDocument,
            ct).ConfigureAwait(false);
    }

    public async Task<PlanningApplication?> GetByUidAsync(string uid, CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(uid);

        var documents = await this.client.QueryAsync(
            CosmosContainerNames.Applications,
            "SELECT * FROM c WHERE c.Uid = @uid",
            [new QueryParameter("@uid", uid)],
            partitionKey: null,
            CosmosJsonSerializerContext.Default.PlanningApplicationDocument,
            ct).ConfigureAwait(false);

        return documents.Count > 0 ? documents[0].ToDomain() : null;
    }

    public async Task<IReadOnlyCollection<PlanningApplication>> GetByAuthorityIdAsync(int authorityId, CancellationToken ct)
    {
        var authorityCode = authorityId.ToString(System.Globalization.CultureInfo.InvariantCulture);

        var documents = await this.client.QueryAsync(
            CosmosContainerNames.Applications,
            "SELECT * FROM c",
            parameters: null,
            authorityCode,
            CosmosJsonSerializerContext.Default.PlanningApplicationDocument,
            ct).ConfigureAwait(false);

        return documents.ConvertAll(doc => doc.ToDomain());
    }

    public async Task<IReadOnlyCollection<PlanningApplication>> FindNearbyAsync(
        string authorityCode, double latitude, double longitude, double radiusMetres, CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(authorityCode);

        const string sql =
            "SELECT * FROM c WHERE ST_DISTANCE(c.location, " +
            "{\"type\": \"Point\", \"coordinates\": [@lng, @lat]}) <= @radius";

        var documents = await this.client.QueryAsync(
            CosmosContainerNames.Applications,
            sql,
            [
                new QueryParameter("@lng", longitude),
                new QueryParameter("@lat", latitude),
                new QueryParameter("@radius", radiusMetres),
            ],
            authorityCode,
            CosmosJsonSerializerContext.Default.PlanningApplicationDocument,
            ct).ConfigureAwait(false);

        return documents.ConvertAll(doc => doc.ToDomain());
    }
}
