using Microsoft.Azure.Cosmos;
using TownCrier.Application.Groups;
using TownCrier.Domain.Groups;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Groups;

public sealed class CosmosGroupInvitationRepository : IGroupInvitationRepository
{
    private readonly Container container;

    public CosmosGroupInvitationRepository(CosmosClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.container = client.GetContainer(CosmosContainerNames.DatabaseName, CosmosContainerNames.Groups);
    }

    public async Task<GroupInvitation?> GetByIdAsync(string invitationId, CancellationToken ct)
    {
        var query = new QueryDefinition(
            "SELECT * FROM c WHERE c.id = @id AND c.type = 'invitation'")
            .WithParameter("@id", GroupInvitationDocument.ToDocumentId(invitationId));

        using var iterator = this.container.GetItemQueryIterator<GroupInvitationDocument>(
            query,
            requestOptions: new QueryRequestOptions { MaxItemCount = 1 });

        return await iterator.FirstOrDefaultAsync(doc => doc.ToDomain(), ct).ConfigureAwait(false);
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

        using var iterator = this.container.GetItemQueryIterator<GroupInvitationDocument>(query);

        return await iterator.CollectAsync(doc => doc.ToDomain(), ct).ConfigureAwait(false);
    }

    public async Task<IReadOnlyList<GroupInvitation>> GetPendingByEmailAsync(string email, CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(email);

#pragma warning disable CA1308 // Emails are normalized to lowercase per industry convention
        var normalizedEmail = email.Trim().ToLowerInvariant();
#pragma warning restore CA1308

        var query = new QueryDefinition(
            "SELECT * FROM c WHERE c.type = 'invitation' AND LOWER(c.inviteeEmail) = @email AND c.status = 'Pending'")
            .WithParameter("@email", normalizedEmail);

        using var iterator = this.container.GetItemQueryIterator<GroupInvitationDocument>(query);

        return await iterator.CollectAsync(doc => doc.ToDomain(), ct).ConfigureAwait(false);
    }
}
