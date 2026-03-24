using System.Net;
using Microsoft.Azure.Cosmos;
using TownCrier.Application.PlanningApplications;
using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Infrastructure.PlanningApplications;

public sealed class CosmosPlanningApplicationRepository : IPlanningApplicationRepository
{
    private readonly Container container;

    public CosmosPlanningApplicationRepository(CosmosClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.container = client.GetContainer("town-crier", "Applications");
    }

    public async Task UpsertAsync(PlanningApplication application, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(application);

        var document = PlanningApplicationDocument.FromDomain(application);
        await this.container.UpsertItemAsync(
            document,
            new PartitionKey(document.AuthorityCode),
            cancellationToken: ct).ConfigureAwait(false);
    }

    public async Task<IReadOnlyCollection<PlanningApplication>> FindNearbyAsync(
        string authorityCode, double latitude, double longitude, double radiusMetres, CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(authorityCode);

        var query = new QueryDefinition(
            "SELECT * FROM c WHERE ST_DISTANCE(c.location, {\"type\": \"Point\", \"coordinates\": [@lng, @lat]}) <= @radius")
            .WithParameter("@lng", longitude)
            .WithParameter("@lat", latitude)
            .WithParameter("@radius", radiusMetres);

        var requestOptions = new QueryRequestOptions
        {
            PartitionKey = new PartitionKey(authorityCode),
        };

        var results = new List<PlanningApplication>();
        using var iterator = this.container.GetItemQueryIterator<PlanningApplicationDocument>(query, requestOptions: requestOptions);

        while (iterator.HasMoreResults)
        {
            var response = await iterator.ReadNextAsync(ct).ConfigureAwait(false);
            foreach (var document in response)
            {
                results.Add(document.ToDomain());
            }
        }

        return results;
    }
}
