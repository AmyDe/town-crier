using TownCrier.Application.Groups;
using TownCrier.Domain.Groups;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Groups;

public sealed class CosmosGroupRepository : IGroupRepository
{
    private readonly ICosmosRestClient client;

    public CosmosGroupRepository(ICosmosRestClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.client = client;
    }

    public async Task<Group?> GetByIdAsync(string groupId, CancellationToken ct)
    {
        var documents = await this.client.QueryAsync(
            CosmosContainerNames.Groups,
            "SELECT * FROM c WHERE c.id = @id AND c.type = 'group'",
            [new QueryParameter("@id", groupId)],
            partitionKey: null,
            CosmosJsonSerializerContext.Default.GroupDocument,
            ct).ConfigureAwait(false);

        return documents.Count > 0 ? documents[0].ToDomain() : null;
    }

    public async Task SaveAsync(Group group, CancellationToken ct)
    {
        var document = GroupDocument.FromDomain(group);
        await this.client.UpsertDocumentAsync(
            CosmosContainerNames.Groups,
            document,
            document.OwnerId,
            CosmosJsonSerializerContext.Default.GroupDocument,
            ct).ConfigureAwait(false);
    }

    public async Task DeleteAsync(string groupId, CancellationToken ct)
    {
        var documents = await this.client.QueryAsync(
            CosmosContainerNames.Groups,
            "SELECT c.id, c.ownerId FROM c WHERE c.id = @id AND c.type = 'group'",
            [new QueryParameter("@id", groupId)],
            partitionKey: null,
            CosmosJsonSerializerContext.Default.GroupDocument,
            ct).ConfigureAwait(false);

        if (documents.Count > 0)
        {
            var document = documents[0];
            await this.client.DeleteDocumentAsync(
                CosmosContainerNames.Groups,
                document.Id,
                document.OwnerId,
                ct).ConfigureAwait(false);
        }
    }

    public async Task<IReadOnlyList<Group>> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        const string sql =
            "SELECT * FROM c WHERE c.type = 'group' " +
            "AND ARRAY_CONTAINS(c.members, {\"userId\": @userId}, true)";

        var documents = await this.client.QueryAsync(
            CosmosContainerNames.Groups,
            sql,
            [new QueryParameter("@userId", userId)],
            partitionKey: null,
            CosmosJsonSerializerContext.Default.GroupDocument,
            ct).ConfigureAwait(false);

        return documents.ConvertAll(doc => doc.ToDomain());
    }
}
