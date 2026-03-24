using Microsoft.Azure.Cosmos;
using TownCrier.Application.Groups;
using TownCrier.Domain.Groups;

namespace TownCrier.Infrastructure.Groups;

public sealed class CosmosGroupInvitationRepository : IGroupInvitationRepository
{
    private readonly Container container;

    public CosmosGroupInvitationRepository(CosmosClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.container = client.GetContainer("town-crier", "groups");
    }

    public async Task<GroupInvitation?> GetByIdAsync(string invitationId, CancellationToken ct)
    {
        var query = new QueryDefinition(
            "SELECT * FROM c WHERE c.id = @id AND c.type = 'invitation'")
            .WithParameter("@id", invitationId);

        using var iterator = this.container.GetItemQueryIterator<GroupInvitationDocument>(
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

    public async Task SaveAsync(GroupInvitation invitation, CancellationToken ct)
    {
        var document = GroupInvitationDocument.FromDomain(invitation);
        await this.container.UpsertItemAsync(
            document,
            new PartitionKey(document.OwnerId),
            cancellationToken: ct).ConfigureAwait(false);
    }

    public async Task<IReadOnlyList<GroupInvitation>> GetPendingByGroupIdAsync(string groupId, CancellationToken ct)
    {
        var query = new QueryDefinition(
            "SELECT * FROM c WHERE c.type = 'invitation' AND c.groupId = @groupId AND c.status = 'Pending'")
            .WithParameter("@groupId", groupId);

        return await this.QueryInvitationsAsync(query, ct).ConfigureAwait(false);
    }

    public async Task<IReadOnlyList<GroupInvitation>> GetPendingByEmailAsync(string email, CancellationToken ct)
    {
        var query = new QueryDefinition(
            "SELECT * FROM c WHERE c.type = 'invitation' AND c.inviteeEmail = @email AND c.status = 'Pending'")
            .WithParameter("@email", email);

        return await this.QueryInvitationsAsync(query, ct).ConfigureAwait(false);
    }

    private async Task<IReadOnlyList<GroupInvitation>> QueryInvitationsAsync(
        QueryDefinition query,
        CancellationToken ct)
    {
        using var iterator = this.container.GetItemQueryIterator<GroupInvitationDocument>(query);

        var invitations = new List<GroupInvitation>();
        while (iterator.HasMoreResults)
        {
            var response = await iterator.ReadNextAsync(ct).ConfigureAwait(false);
            foreach (var document in response)
            {
                invitations.Add(document.ToDomain());
            }
        }

        return invitations;
    }
}
