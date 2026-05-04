using TownCrier.Application.NotificationState;
using TownCrier.Domain.NotificationState;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.NotificationState;

/// <summary>
/// Cosmos REST adapter for <see cref="INotificationStateRepository"/>. Each user
/// owns one document on the <c>NotificationState</c> container, partitioned on
/// <c>/userId</c> and keyed by the same userId — see
/// <c>docs/specs/notifications-unread-watermark.md#api-domain</c>.
/// </summary>
public sealed class CosmosNotificationStateRepository : INotificationStateRepository
{
    private readonly ICosmosRestClient client;

    public CosmosNotificationStateRepository(ICosmosRestClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.client = client;
    }

    /// <inheritdoc />
    public async Task<NotificationStateAggregate?> GetByUserIdAsync(
        string userId, CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);

        var document = await this.client.ReadDocumentAsync(
            CosmosContainerNames.NotificationState,
            userId,
            userId,
            CosmosJsonSerializerContext.Default.NotificationStateDocument,
            ct).ConfigureAwait(false);

        return document?.ToDomain();
    }

    /// <inheritdoc />
    public async Task SaveAsync(NotificationStateAggregate state, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(state);
        var document = NotificationStateDocument.FromDomain(state);

        await this.client.UpsertDocumentAsync(
            CosmosContainerNames.NotificationState,
            document,
            document.UserId,
            CosmosJsonSerializerContext.Default.NotificationStateDocument,
            ct).ConfigureAwait(false);
    }
}
