using TownCrier.Application.SavedApplications;
using TownCrier.Domain.SavedApplications;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.SavedApplications;

public sealed class CosmosSavedApplicationRepository : ISavedApplicationRepository
{
    private readonly ICosmosRestClient client;

    public CosmosSavedApplicationRepository(ICosmosRestClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.client = client;
    }

    public async Task SaveAsync(SavedApplication savedApplication, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(savedApplication);

        var document = SavedApplicationDocument.FromDomain(savedApplication);
        await this.client.UpsertDocumentAsync(
            CosmosContainerNames.SavedApplications,
            document,
            document.UserId,
            CosmosJsonSerializerContext.Default.SavedApplicationDocument,
            ct).ConfigureAwait(false);
    }

    public async Task DeleteAsync(string userId, string applicationUid, CancellationToken ct)
    {
        var id = SavedApplicationDocument.MakeId(userId, applicationUid);

        await this.client.DeleteDocumentAsync(
            CosmosContainerNames.SavedApplications,
            id,
            userId,
            ct).ConfigureAwait(false);
    }

    public async Task DeleteAllByUserIdAsync(string userId, CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);

        var documents = await this.client.QueryAsync(
            CosmosContainerNames.SavedApplications,
            "SELECT c.id FROM c WHERE c.userId = @userId",
            [new QueryParameter("@userId", userId)],
            userId,
            CosmosJsonSerializerContext.Default.SavedApplicationDocument,
            ct).ConfigureAwait(false);

        foreach (var document in documents)
        {
            await this.client.DeleteDocumentAsync(
                CosmosContainerNames.SavedApplications,
                document.Id,
                userId,
                ct).ConfigureAwait(false);
        }
    }

    public async Task<IReadOnlyList<SavedApplication>> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        var documents = await this.client.QueryAsync(
            CosmosContainerNames.SavedApplications,
            "SELECT * FROM c WHERE c.userId = @userId",
            [new QueryParameter("@userId", userId)],
            userId,
            CosmosJsonSerializerContext.Default.SavedApplicationDocument,
            ct).ConfigureAwait(false);

        return documents.ConvertAll(doc => doc.ToDomain());
    }

    public async Task<bool> ExistsAsync(string userId, string applicationUid, CancellationToken ct)
    {
        var id = SavedApplicationDocument.MakeId(userId, applicationUid);

        var document = await this.client.ReadDocumentAsync(
            CosmosContainerNames.SavedApplications,
            id,
            userId,
            CosmosJsonSerializerContext.Default.SavedApplicationDocument,
            ct).ConfigureAwait(false);

        return document is not null;
    }

    public async Task<IReadOnlyList<string>> GetUserIdsByApplicationUidAsync(string applicationUid, CancellationToken ct)
    {
        return await this.client.QueryAsync(
            CosmosContainerNames.SavedApplications,
            "SELECT VALUE c.userId FROM c WHERE c.applicationUid = @applicationUid",
            [new QueryParameter("@applicationUid", applicationUid)],
            partitionKey: null,
            CosmosJsonSerializerContext.Default.String,
            ct).ConfigureAwait(false);
    }
}
