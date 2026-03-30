using TownCrier.Application.Groups;
using TownCrier.Domain.Groups;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Groups;

public sealed class CosmosGroupInvitationRepository : IGroupInvitationRepository
{
    private readonly ICosmosRestClient client;

    public CosmosGroupInvitationRepository(ICosmosRestClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.client = client;
    }

    public async Task<GroupInvitation?> GetByIdAsync(string invitationId, CancellationToken ct)
    {
        var documents = await this.client.QueryAsync(
            CosmosContainerNames.Groups,
            "SELECT * FROM c WHERE c.id = @id AND c.type = 'invitation'",
            [new QueryParameter("@id", GroupInvitationDocument.ToDocumentId(invitationId))],
            partitionKey: null,
            CosmosJsonSerializerContext.Default.GroupInvitationDocument,
            ct).ConfigureAwait(false);

        return documents.Count > 0 ? documents[0].ToDomain() : null;
    }

    public async Task SaveAsync(GroupInvitation invitation, CancellationToken ct)
    {
        var document = GroupInvitationDocument.FromDomain(invitation);
        await this.client.UpsertDocumentAsync(
            CosmosContainerNames.Groups,
            document,
            document.OwnerId,
            CosmosJsonSerializerContext.Default.GroupInvitationDocument,
            ct).ConfigureAwait(false);
    }

    public async Task<IReadOnlyList<GroupInvitation>> GetPendingByGroupIdAsync(string groupId, CancellationToken ct)
    {
        var documents = await this.client.QueryAsync(
            CosmosContainerNames.Groups,
            "SELECT * FROM c WHERE c.type = 'invitation' AND c.groupId = @groupId AND c.status = 'Pending'",
            [new QueryParameter("@groupId", groupId)],
            partitionKey: null,
            CosmosJsonSerializerContext.Default.GroupInvitationDocument,
            ct).ConfigureAwait(false);

        return documents.ConvertAll(doc => doc.ToDomain());
    }

    public async Task<IReadOnlyList<GroupInvitation>> GetPendingByEmailAsync(string email, CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(email);

#pragma warning disable CA1308 // Emails are normalized to lowercase per industry convention
        var normalizedEmail = email.Trim().ToLowerInvariant();
#pragma warning restore CA1308

        var documents = await this.client.QueryAsync(
            CosmosContainerNames.Groups,
            "SELECT * FROM c WHERE c.type = 'invitation' AND LOWER(c.inviteeEmail) = @email AND c.status = 'Pending'",
            [new QueryParameter("@email", normalizedEmail)],
            partitionKey: null,
            CosmosJsonSerializerContext.Default.GroupInvitationDocument,
            ct).ConfigureAwait(false);

        return documents.ConvertAll(doc => doc.ToDomain());
    }
}
