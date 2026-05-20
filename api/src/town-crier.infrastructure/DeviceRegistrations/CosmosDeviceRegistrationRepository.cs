using TownCrier.Application.DeviceRegistrations;
using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.DeviceRegistrations;

public sealed class CosmosDeviceRegistrationRepository : IDeviceRegistrationRepository
{
    private readonly ICosmosRestClient client;

    public CosmosDeviceRegistrationRepository(ICosmosRestClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.client = client;
    }

    public async Task<DeviceRegistration?> GetByTokenAsync(string userId, string token, CancellationToken ct)
    {
        // Point read scoped to the user's partition (userId = partition key, token = document id).
        // This replaces the former cross-partition scan and costs ~1 RU instead of O(partitions) RUs.
        var document = await this.client.ReadDocumentAsync(
            CosmosContainerNames.DeviceRegistrations,
            token,
            userId,
            CosmosJsonSerializerContext.Default.DeviceRegistrationDocument,
            ct).ConfigureAwait(false);

        return document?.ToDomain();
    }

    public async Task<IReadOnlyList<DeviceRegistration>> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        var documents = await this.client.QueryAsync(
            CosmosContainerNames.DeviceRegistrations,
            "SELECT * FROM c WHERE c.userId = @userId",
            [new QueryParameter("@userId", userId)],
            userId,
            CosmosJsonSerializerContext.Default.DeviceRegistrationDocument,
            ct).ConfigureAwait(false);

        return documents.ConvertAll(doc => doc.ToDomain());
    }

    public async Task SaveAsync(DeviceRegistration registration, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(registration);

        var document = DeviceRegistrationDocument.FromDomain(registration);

        await this.client.UpsertDocumentAsync(
            CosmosContainerNames.DeviceRegistrations,
            document,
            document.UserId,
            CosmosJsonSerializerContext.Default.DeviceRegistrationDocument,
            ct).ConfigureAwait(false);
    }

    public async Task DeleteByTokenAsync(string userId, string token, CancellationToken ct)
    {
        // Direct partitioned delete — document id is the token, partition key is the userId.
        // No cross-partition lookup needed. Idempotent: no error if the document is absent
        // (token already cleaned up by TTL or a prior call).
        //
        // Orphan-row note: if a token is reassigned from user A to user B (device shared,
        // app reinstalled under a different account), user A's row is left in place.
        // It will be collected by the APNs invalid-token callback or by the 180-day TTL.
        // This is accepted per the design decision in GH#395.
        await this.client.DeleteDocumentAsync(
            CosmosContainerNames.DeviceRegistrations,
            token,
            userId,
            ct).ConfigureAwait(false);
    }

    public async Task DeleteAllByUserIdAsync(string userId, CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);

        var documents = await this.client.QueryAsync(
            CosmosContainerNames.DeviceRegistrations,
            "SELECT c.id FROM c WHERE c.userId = @userId",
            [new QueryParameter("@userId", userId)],
            userId,
            CosmosJsonSerializerContext.Default.DeviceRegistrationDocument,
            ct).ConfigureAwait(false);

        foreach (var document in documents)
        {
            await this.client.DeleteDocumentAsync(
                CosmosContainerNames.DeviceRegistrations,
                document.Id,
                userId,
                ct).ConfigureAwait(false);
        }
    }
}
