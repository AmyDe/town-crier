using System.Net;
using Microsoft.Azure.Cosmos;
using TownCrier.Application.DecisionAlerts;
using TownCrier.Domain.DecisionAlerts;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.DecisionAlerts;

public sealed class CosmosDecisionAlertRepository : IDecisionAlertRepository
{
    private readonly Container container;

    public CosmosDecisionAlertRepository(CosmosClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.container = client.GetContainer(CosmosContainerNames.DatabaseName, CosmosContainerNames.DecisionAlerts);
    }

    public async Task<DecisionAlert?> GetByUserAndApplicationAsync(
        string userId, string applicationUid, CancellationToken ct)
    {
        var documentId = DecisionAlertDocument.MakeId(userId, applicationUid);

        try
        {
            var response = await this.container.ReadItemAsync<DecisionAlertDocument>(
                documentId,
                new PartitionKey(userId),
                cancellationToken: ct).ConfigureAwait(false);

            return response.Resource.ToDomain();
        }
        catch (CosmosException ex) when (ex.StatusCode == HttpStatusCode.NotFound)
        {
            return null;
        }
    }

    public async Task SaveAsync(DecisionAlert alert, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(alert);

        var document = DecisionAlertDocument.FromDomain(alert);

        await this.container.UpsertItemAsync(
            document,
            new PartitionKey(document.UserId),
            cancellationToken: ct).ConfigureAwait(false);
    }
}
