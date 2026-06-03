using TownCrier.Application.Notifications;
using TownCrier.Domain.Notifications;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Notifications;

public sealed class CosmosNotificationRepository : INotificationRepository
{
    private static readonly IReadOnlyDictionary<string, Notification> EmptyMap =
        new Dictionary<string, Notification>(StringComparer.Ordinal);

    private readonly ICosmosRestClient client;

    public CosmosNotificationRepository(ICosmosRestClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.client = client;
    }

    public async Task<Notification?> GetByUserAndApplicationAsync(
        string userId,
        string applicationUid,
        int authorityId,
        NotificationEventType eventType,
        CancellationToken ct)
    {
        // Idempotency key includes authorityId because PlanIt uids collide
        // across councils (bd tc-th98 / GH#384). A user can hold one notification
        // per (uid, authorityId, eventType) but two different councils' rows must
        // not suppress each other.
        var documents = await this.client.QueryAsync(
            CosmosContainerNames.Notifications,
            "SELECT * FROM c WHERE c.userId = @userId AND c.applicationUid = @appUid AND c.authorityId = @authorityId AND c.eventType = @eventType",
            [
                new QueryParameter("@userId", userId),
                new QueryParameter("@appUid", applicationUid),
                new QueryParameter("@authorityId", authorityId),
                new QueryParameter("@eventType", eventType.ToString()),
            ],
            userId,
            CosmosJsonSerializerContext.Default.NotificationDocument,
            ct).ConfigureAwait(false);

        return documents.Count > 0 ? documents[0].ToDomain() : null;
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

    public async Task<int> GetUnreadCountAsync(
        string userId, DateTimeOffset lastReadAt, CancellationToken ct)
    {
        // Strictly-greater-than: a notification created at exactly lastReadAt is
        // considered read (the watermark is the cutoff itself). Mirrors the
        // FakeNotificationRepository implementation and the spec's read model.
        return await this.client.ScalarQueryAsync(
            CosmosContainerNames.Notifications,
            "SELECT VALUE COUNT(1) FROM c WHERE c.userId = @userId AND c.createdAt > @lastReadAt",
            [new QueryParameter("@userId", userId), new QueryParameter("@lastReadAt", lastReadAt)],
            userId,
            CosmosJsonSerializerContext.Default.Int32,
            ct).ConfigureAwait(false);
    }

    public async Task<IReadOnlyDictionary<string, Notification>> GetLatestUnreadByApplicationsAsync(
        string userId,
        IReadOnlyCollection<string> applicationUids,
        DateTimeOffset lastReadAt,
        CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(applicationUids);

        if (applicationUids.Count == 0)
        {
            return EmptyMap;
        }

        // Single round-trip for every uid in the zone. ARRAY_CONTAINS binds the uid
        // set as one parameter; partitioned by userId so this stays single-partition.
        // We fetch all unread rows for the set, then reduce to the latest per uid in
        // memory — collapsing the former per-application N+1 loop (bd tc-1wkp).
        var uids = applicationUids as string[] ?? [.. applicationUids];

        const string sql = "SELECT * FROM c "
            + "WHERE c.userId = @userId AND ARRAY_CONTAINS(@uids, c.applicationUid) "
            + "AND c.createdAt > @lastReadAt "
            + "ORDER BY c.createdAt DESC";

        var documents = await this.client.QueryAsync(
            CosmosContainerNames.Notifications,
            sql,
            [
                new QueryParameter("@userId", userId),
                new QueryParameter("@uids", uids),
                new QueryParameter("@lastReadAt", lastReadAt),
            ],
            userId,
            CosmosJsonSerializerContext.Default.NotificationDocument,
            ct).ConfigureAwait(false);

        // Rows arrive newest-first, so the first row seen per uid is the latest.
        var map = new Dictionary<string, Notification>(StringComparer.Ordinal);
        foreach (var document in documents)
        {
            var notification = document.ToDomain();
            map.TryAdd(notification.ApplicationUid, notification);
        }

        return map;
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

    public async Task<IReadOnlyList<string>> GetUserIdsWithUnsentEmailsCrossPartitionAsync(CancellationToken ct)
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
