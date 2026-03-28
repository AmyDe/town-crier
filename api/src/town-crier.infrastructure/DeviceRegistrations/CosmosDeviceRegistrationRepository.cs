using System.Net;
using Microsoft.Azure.Cosmos;
using TownCrier.Application.DeviceRegistrations;
using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.DeviceRegistrations;

public sealed class CosmosDeviceRegistrationRepository : IDeviceRegistrationRepository
{
    private readonly Container container;

    public CosmosDeviceRegistrationRepository(CosmosClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.container = client.GetContainer("town-crier", "DeviceRegistrations");
    }

    public async Task<DeviceRegistration?> GetByTokenAsync(string token, CancellationToken ct)
    {
        var query = new QueryDefinition("SELECT * FROM c WHERE c.token = @token")
            .WithParameter("@token", token);

        using var iterator = this.container.GetItemQueryIterator<DeviceRegistrationDocument>(
            query,
            requestOptions: new QueryRequestOptions { MaxItemCount = 1 });

        return await iterator.FirstOrDefaultAsync(doc => doc.ToDomain(), ct).ConfigureAwait(false);
    }

    public async Task<IReadOnlyList<DeviceRegistration>> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        var query = new QueryDefinition("SELECT * FROM c WHERE c.userId = @userId")
            .WithParameter("@userId", userId);

        using var iterator = this.container.GetItemQueryIterator<DeviceRegistrationDocument>(
            query,
            requestOptions: new QueryRequestOptions
            {
                PartitionKey = new PartitionKey(userId),
            });

        return await iterator.CollectAsync(doc => doc.ToDomain(), ct).ConfigureAwait(false);
    }

    public async Task SaveAsync(DeviceRegistration registration, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(registration);

        var document = DeviceRegistrationDocument.FromDomain(registration);

        await this.container.UpsertItemAsync(
            document,
            new PartitionKey(document.UserId),
            cancellationToken: ct).ConfigureAwait(false);
    }

    public async Task DeleteByTokenAsync(string token, CancellationToken ct)
    {
        // Token is not the partition key, so we must find the document first
        // to get the userId (partition key) needed for deletion.
        var query = new QueryDefinition("SELECT c.id, c.userId FROM c WHERE c.token = @token")
            .WithParameter("@token", token);

        using var iterator = this.container.GetItemQueryIterator<DeviceRegistrationDocument>(query);

        var documents = await iterator.CollectAsync(ct).ConfigureAwait(false);

        foreach (var document in documents)
        {
            try
            {
                await this.container.DeleteItemAsync<DeviceRegistrationDocument>(
                    document.Id,
                    new PartitionKey(document.UserId),
                    cancellationToken: ct).ConfigureAwait(false);
            }
            catch (CosmosException ex) when (ex.StatusCode == HttpStatusCode.NotFound)
            {
                // Already deleted — idempotent
            }
        }
    }
}
