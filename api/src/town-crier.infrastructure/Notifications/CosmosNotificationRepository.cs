using Microsoft.Azure.Cosmos;
using TownCrier.Application.Notifications;
using TownCrier.Domain.Notifications;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Notifications;

public sealed class CosmosNotificationRepository : INotificationRepository
{
    private readonly Container container;

    public CosmosNotificationRepository(CosmosClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.container = client.GetContainer("town-crier", "notifications");
    }

    public async Task<Notification?> GetByUserAndApplicationAsync(
        string userId, string applicationName, CancellationToken ct)
    {
        var query = new QueryDefinition(
            "SELECT * FROM c WHERE c.userId = @userId AND c.applicationName = @appName")
            .WithParameter("@userId", userId)
            .WithParameter("@appName", applicationName);

        using var iterator = this.container.GetItemQueryIterator<NotificationDocument>(
            query,
            requestOptions: new QueryRequestOptions
            {
                PartitionKey = new PartitionKey(userId),
            });

        return await iterator.FirstOrDefaultAsync(doc => doc.ToDomain(), ct).ConfigureAwait(false);
    }

    public async Task<int> CountByUserInMonthAsync(
        string userId, int year, int month, CancellationToken ct)
    {
        var startOfMonth = new DateTimeOffset(year, month, 1, 0, 0, 0, TimeSpan.Zero);
        var startOfNextMonth = startOfMonth.AddMonths(1);

        var query = new QueryDefinition(
            "SELECT VALUE COUNT(1) FROM c WHERE c.userId = @userId AND c.createdAt >= @start AND c.createdAt < @end")
            .WithParameter("@userId", userId)
            .WithParameter("@start", startOfMonth)
            .WithParameter("@end", startOfNextMonth);

        using var iterator = this.container.GetItemQueryIterator<int>(
            query,
            requestOptions: new QueryRequestOptions
            {
                PartitionKey = new PartitionKey(userId),
            });

        return await iterator.ScalarAsync(ct).ConfigureAwait(false);
    }

    public async Task<int> CountByUserSinceAsync(
        string userId, DateTimeOffset since, CancellationToken ct)
    {
        var query = new QueryDefinition(
            "SELECT VALUE COUNT(1) FROM c WHERE c.userId = @userId AND c.createdAt >= @since")
            .WithParameter("@userId", userId)
            .WithParameter("@since", since);

        using var iterator = this.container.GetItemQueryIterator<int>(
            query,
            requestOptions: new QueryRequestOptions
            {
                PartitionKey = new PartitionKey(userId),
            });

        return await iterator.ScalarAsync(ct).ConfigureAwait(false);
    }

    public async Task<(IReadOnlyList<Notification> Items, int Total)> GetByUserPaginatedAsync(
        string userId, int page, int pageSize, CancellationToken ct)
    {
        // Count total notifications for this user
        var countQuery = new QueryDefinition(
            "SELECT VALUE COUNT(1) FROM c WHERE c.userId = @userId")
            .WithParameter("@userId", userId);

        int total;
        using (var countIterator = this.container.GetItemQueryIterator<int>(
            countQuery,
            requestOptions: new QueryRequestOptions
            {
                PartitionKey = new PartitionKey(userId),
            }))
        {
            total = await countIterator.ScalarAsync(ct).ConfigureAwait(false);
        }

        if (total == 0)
        {
            return (Array.Empty<Notification>(), 0);
        }

        // Fetch page — reverse-chronological by _ts
        var offset = (page - 1) * pageSize;
        var itemsQuery = new QueryDefinition(
            "SELECT * FROM c WHERE c.userId = @userId ORDER BY c._ts DESC OFFSET @offset LIMIT @limit")
            .WithParameter("@userId", userId)
            .WithParameter("@offset", offset)
            .WithParameter("@limit", pageSize);

        using var itemsIterator = this.container.GetItemQueryIterator<NotificationDocument>(
            itemsQuery,
            requestOptions: new QueryRequestOptions
            {
                PartitionKey = new PartitionKey(userId),
            });

        var items = await itemsIterator.CollectAsync(doc => doc.ToDomain(), ct).ConfigureAwait(false);

        return (items, total);
    }

    public async Task SaveAsync(Notification notification, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(notification);
        var document = NotificationDocument.FromDomain(notification);

        await this.container.UpsertItemAsync(
            document,
            new PartitionKey(document.UserId),
            cancellationToken: ct).ConfigureAwait(false);
    }
}
