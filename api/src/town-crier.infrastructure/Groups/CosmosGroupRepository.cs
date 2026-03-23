using Microsoft.Azure.Cosmos;
using TownCrier.Application.Groups;
using TownCrier.Domain.Groups;

namespace TownCrier.Infrastructure.Groups;

public sealed class CosmosGroupRepository : IGroupRepository
{
    private readonly Container container;

    public CosmosGroupRepository(CosmosClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.container = client.GetContainer("town-crier", "groups");
    }

    public async Task<Group?> GetByIdAsync(string groupId, CancellationToken ct)
    {
        var query = new QueryDefinition(
            "SELECT * FROM c WHERE c.id = @id AND c.type = 'group'")
            .WithParameter("@id", groupId);

        using var iterator = this.container.GetItemQueryIterator<GroupDocument>(
            query,
            requestOptions: new QueryRequestOptions { MaxItemCount = 1 });

        while (iterator.HasMoreResults)
        {
            var response = await iterator.ReadNextAsync(ct).ConfigureAwait(false);
            var document = response.FirstOrDefault();
            if (document is not null)
            {
                return document.ToDomain();
            }
        }

        return null;
    }

    public async Task SaveAsync(Group group, CancellationToken ct)
    {
        var document = GroupDocument.FromDomain(group);
        await this.container.UpsertItemAsync(
            document,
            new PartitionKey(document.OwnerId),
            cancellationToken: ct).ConfigureAwait(false);
    }

    public async Task DeleteAsync(string groupId, CancellationToken ct)
    {
        var query = new QueryDefinition(
            "SELECT c.id, c.ownerId FROM c WHERE c.id = @id AND c.type = 'group'")
            .WithParameter("@id", groupId);

        using var iterator = this.container.GetItemQueryIterator<GroupDocument>(
            query,
            requestOptions: new QueryRequestOptions { MaxItemCount = 1 });

        while (iterator.HasMoreResults)
        {
            var response = await iterator.ReadNextAsync(ct).ConfigureAwait(false);
            var document = response.FirstOrDefault();
            if (document is not null)
            {
                await this.container.DeleteItemAsync<GroupDocument>(
                    document.Id,
                    new PartitionKey(document.OwnerId),
                    cancellationToken: ct).ConfigureAwait(false);
                return;
            }
        }
    }

    public async Task<IReadOnlyList<Group>> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        var query = new QueryDefinition(
            "SELECT * FROM c WHERE c.type = 'group' AND ARRAY_CONTAINS(c.members, {\"userId\": @userId}, true)")
            .WithParameter("@userId", userId);

        using var iterator = this.container.GetItemQueryIterator<GroupDocument>(query);

        var groups = new List<Group>();
        while (iterator.HasMoreResults)
        {
            var response = await iterator.ReadNextAsync(ct).ConfigureAwait(false);
            foreach (var document in response)
            {
                groups.Add(document.ToDomain());
            }
        }

        return groups;
    }
}
