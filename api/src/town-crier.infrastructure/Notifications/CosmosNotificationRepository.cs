using TownCrier.Application.Notifications;
using TownCrier.Domain.Notifications;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Notifications;

public sealed class CosmosNotificationRepository : INotificationRepository
{
    private readonly ICosmosRestClient client;

    public CosmosNotificationRepository(ICosmosRestClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.client = client;
    }

    public async Task<Notification?> GetByUserAndApplicationAsync(
        string userId, string applicationName, CancellationToken ct)
    {
        var documents = await this.client.QueryAsync(
            CosmosContainerNames.Notifications,
            "SELECT * FROM c WHERE c.userId = @userId AND c.applicationName = @appName",
            [new QueryParameter("@userId", userId), new QueryParameter("@appName", applicationName)],
            userId,
            CosmosJsonSerializerContext.Default.NotificationDocument,
            ct).ConfigureAwait(false);

        return documents.Count > 0 ? documents[0].ToDomain() : null;
    }

    public async Task<int> CountByUserInMonthAsync(
        string userId, int year, int month, CancellationToken ct)
    {
        var startOfMonth = new DateTimeOffset(year, month, 1, 0, 0, 0, TimeSpan.Zero);
        var startOfNextMonth = startOfMonth.AddMonths(1);

        return await this.client.ScalarQueryAsync(
            CosmosContainerNames.Notifications,
            "SELECT VALUE COUNT(1) FROM c WHERE c.userId = @userId AND c.createdAt >= @start AND c.createdAt < @end",
            [
                new QueryParameter("@userId", userId),
                new QueryParameter("@start", startOfMonth),
                new QueryParameter("@end", startOfNextMonth),
            ],
            userId,
            CosmosJsonSerializerContext.Default.Int32,
            ct).ConfigureAwait(false);
    }

    public async Task<int> CountByUserSinceAsync(
        string userId, DateTimeOffset since, CancellationToken ct)
    {
        return await this.client.ScalarQueryAsync(
            CosmosContainerNames.Notifications,
            "SELECT VALUE COUNT(1) FROM c WHERE c.userId = @userId AND c.createdAt >= @since",
            [new QueryParameter("@userId", userId), new QueryParameter("@since", since)],
            userId,
            CosmosJsonSerializerContext.Default.Int32,
            ct).ConfigureAwait(false);
    }

    public async Task<IReadOnlyList<Notification>> GetByUserSinceAsync(
        string userId, DateTimeOffset since, CancellationToken ct)
    {
        var documents = await this.client.QueryAsync(
            CosmosContainerNames.Notifications,
            "SELECT * FROM c WHERE c.userId = @userId AND c.createdAt >= @since ORDER BY c.createdAt DESC",
            [new QueryParameter("@userId", userId), new QueryParameter("@since", since)],
            userId,
            CosmosJsonSerializerContext.Default.NotificationDocument,
            ct).ConfigureAwait(false);

        return documents.ConvertAll(doc => doc.ToDomain());
    }

    public async Task<(IReadOnlyList<Notification> Items, int Total)> GetByUserPaginatedAsync(
        string userId, int page, int pageSize, CancellationToken ct)
    {
        // Count total notifications for this user
        var total = await this.client.ScalarQueryAsync(
            CosmosContainerNames.Notifications,
            "SELECT VALUE COUNT(1) FROM c WHERE c.userId = @userId",
            [new QueryParameter("@userId", userId)],
            userId,
            CosmosJsonSerializerContext.Default.Int32,
            ct).ConfigureAwait(false);

        if (total == 0)
        {
            return (Array.Empty<Notification>(), 0);
        }

        // Fetch page -- reverse-chronological by _ts
        var offset = (page - 1) * pageSize;

        var documents = await this.client.QueryAsync(
            CosmosContainerNames.Notifications,
            "SELECT * FROM c WHERE c.userId = @userId ORDER BY c._ts DESC OFFSET @offset LIMIT @limit",
            [
                new QueryParameter("@userId", userId),
                new QueryParameter("@offset", offset),
                new QueryParameter("@limit", pageSize),
            ],
            userId,
            CosmosJsonSerializerContext.Default.NotificationDocument,
            ct).ConfigureAwait(false);

        var items = documents.ConvertAll(doc => doc.ToDomain());

        return (items, total);
    }

    public async Task<IReadOnlyList<Notification>> GetUnsentEmailsByUserAsync(
        string userId, CancellationToken ct)
    {
        var documents = await this.client.QueryAsync(
            CosmosContainerNames.Notifications,
            "SELECT * FROM c WHERE c.userId = @userId AND (c.emailSent = false OR NOT IS_DEFINED(c.emailSent)) ORDER BY c.createdAt ASC",
            [new QueryParameter("@userId", userId)],
            userId,
            CosmosJsonSerializerContext.Default.NotificationDocument,
            ct).ConfigureAwait(false);

        return documents.ConvertAll(doc => doc.ToDomain());
    }

    public async Task<IReadOnlyList<string>> GetUserIdsWithUnsentEmailsAsync(CancellationToken ct)
    {
        var userIds = await this.client.QueryAsync(
            CosmosContainerNames.Notifications,
            "SELECT DISTINCT VALUE c.userId FROM c WHERE c.emailSent = false OR NOT IS_DEFINED(c.emailSent)",
            null,
            null,
            CosmosJsonSerializerContext.Default.String,
            ct).ConfigureAwait(false);

        return userIds;
    }

    public async Task SaveAsync(Notification notification, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(notification);
        var document = NotificationDocument.FromDomain(notification);

        await this.client.UpsertDocumentAsync(
            CosmosContainerNames.Notifications,
            document,
            document.UserId,
            CosmosJsonSerializerContext.Default.NotificationDocument,
            ct).ConfigureAwait(false);
    }

    public async Task DeleteAllByUserIdAsync(string userId, CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);

        var documents = await this.client.QueryAsync(
            CosmosContainerNames.Notifications,
            "SELECT c.id FROM c WHERE c.userId = @userId",
            [new QueryParameter("@userId", userId)],
            userId,
            CosmosJsonSerializerContext.Default.NotificationDocument,
            ct).ConfigureAwait(false);

        foreach (var document in documents)
        {
            await this.client.DeleteDocumentAsync(
                CosmosContainerNames.Notifications,
                document.Id,
                userId,
                ct).ConfigureAwait(false);
        }
    }
}
