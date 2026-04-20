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

    public async Task<DeviceRegistration?> GetByTokenAsync(string token, CancellationToken ct)
    {
        var documents = await this.client.QueryAsync(
            CosmosContainerNames.DeviceRegistrations,
            "SELECT * FROM c WHERE c.token = @token",
            [new QueryParameter("@token", token)],
            partitionKey: null,
            CosmosJsonSerializerContext.Default.DeviceRegistrationDocument,
            ct).ConfigureAwait(false);

        return documents.Count > 0 ? documents[0].ToDomain() : null;
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

    public async Task DeleteByTokenAsync(string token, CancellationToken ct)
    {
        // Token is not the partition key, so we must find the document first
        // to get the userId (partition key) needed for deletion.
        var documents = await this.client.QueryAsync(
            CosmosContainerNames.DeviceRegistrations,
            "SELECT c.id, c.userId FROM c WHERE c.token = @token",
            [new QueryParameter("@token", token)],
            partitionKey: null,
            CosmosJsonSerializerContext.Default.DeviceRegistrationDocument,
            ct).ConfigureAwait(false);

        foreach (var document in documents)
        {
            await this.client.DeleteDocumentAsync(
                CosmosContainerNames.DeviceRegistrations,
                document.Id,
                document.UserId,
                ct).ConfigureAwait(false);
        }
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
