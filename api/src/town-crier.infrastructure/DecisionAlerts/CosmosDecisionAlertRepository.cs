using TownCrier.Application.DecisionAlerts;
using TownCrier.Domain.DecisionAlerts;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.DecisionAlerts;

public sealed class CosmosDecisionAlertRepository : IDecisionAlertRepository
{
    private readonly ICosmosRestClient client;

    public CosmosDecisionAlertRepository(ICosmosRestClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.client = client;
    }

    public async Task<DecisionAlert?> GetByUserAndApplicationAsync(
        string userId, string applicationUid, CancellationToken ct)
    {
        var documentId = DecisionAlertDocument.MakeId(userId, applicationUid);

        var document = await this.client.ReadDocumentAsync(
            CosmosContainerNames.DecisionAlerts,
            documentId,
            userId,
            CosmosJsonSerializerContext.Default.DecisionAlertDocument,
            ct).ConfigureAwait(false);

        return document?.ToDomain();
    }

    public async Task<IReadOnlyList<DecisionAlert>> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);

        var documents = await this.client.QueryAsync(
            CosmosContainerNames.DecisionAlerts,
            "SELECT * FROM c WHERE c.userId = @userId",
            [new QueryParameter("@userId", userId)],
            userId,
            CosmosJsonSerializerContext.Default.DecisionAlertDocument,
            ct).ConfigureAwait(false);

        return documents.ConvertAll(doc => doc.ToDomain());
    }

    public async Task SaveAsync(DecisionAlert alert, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(alert);

        var document = DecisionAlertDocument.FromDomain(alert);

        await this.client.UpsertDocumentAsync(
            CosmosContainerNames.DecisionAlerts,
            document,
            document.UserId,
            CosmosJsonSerializerContext.Default.DecisionAlertDocument,
            ct).ConfigureAwait(false);
    }

    public async Task DeleteAllByUserIdAsync(string userId, CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);

        var documents = await this.client.QueryAsync(
            CosmosContainerNames.DecisionAlerts,
            "SELECT c.id FROM c WHERE c.userId = @userId",
            [new QueryParameter("@userId", userId)],
            userId,
            CosmosJsonSerializerContext.Default.DecisionAlertDocument,
            ct).ConfigureAwait(false);

        foreach (var document in documents)
        {
            await this.client.DeleteDocumentAsync(
                CosmosContainerNames.DecisionAlerts,
                document.Id,
                userId,
                ct).ConfigureAwait(false);
        }
    }
}
