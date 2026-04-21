using System.Globalization;
using TownCrier.Application.Polling;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Polling;

/// <summary>
/// Best-effort <see cref="IPollingLeaseStore"/> backed by the Cosmos
/// <c>Leases</c> container. A single document with id <c>"polling"</c> gates
/// concurrent poll cycles between the Service Bus-triggered worker and the
/// safety-net cron.
///
/// Acquisition reads the current lease; if absent or past <c>ExpiresAtUtc</c>
/// the store upserts a fresh one and reports success. There is a residual
/// read-after-write race (two workers acquiring simultaneously) which matches
/// the "best-effort" contract on the interface — duplicate polls are bounded
/// by the lease TTL and PlanIt's per-authority idempotency.
/// </summary>
public sealed class CosmosPollingLeaseStore : IPollingLeaseStore
{
    private const string LeaseDocumentId = "polling";

    private readonly ICosmosRestClient client;
    private readonly TimeProvider timeProvider;
    private readonly string holderId;

    public CosmosPollingLeaseStore(ICosmosRestClient client, TimeProvider timeProvider)
    {
        ArgumentNullException.ThrowIfNull(client);
        ArgumentNullException.ThrowIfNull(timeProvider);

        this.client = client;
        this.timeProvider = timeProvider;

        // One holderId per process instance. Written as diagnostic metadata; lease
        // decisions compare ExpiresAtUtc, not holder identity.
        this.holderId = Guid.NewGuid().ToString("N");
    }

    public async Task<bool> TryAcquireAsync(TimeSpan ttl, CancellationToken ct)
    {
        var now = this.timeProvider.GetUtcNow();

        var existing = await this.client.ReadDocumentAsync(
            CosmosContainerNames.Leases,
            LeaseDocumentId,
            LeaseDocumentId,
            CosmosJsonSerializerContext.Default.PollingLeaseDocument,
            ct).ConfigureAwait(false);

        if (existing is not null && TryParseExpiry(existing.ExpiresAtUtc, out var expiresAt) && expiresAt > now)
        {
            return false;
        }

        var document = new PollingLeaseDocument
        {
            Id = LeaseDocumentId,
            HolderId = this.holderId,
            ExpiresAtUtc = (now + ttl).ToString("o", CultureInfo.InvariantCulture),
        };

        await this.client.UpsertDocumentAsync(
            CosmosContainerNames.Leases,
            document,
            LeaseDocumentId,
            CosmosJsonSerializerContext.Default.PollingLeaseDocument,
            ct).ConfigureAwait(false);

        return true;
    }

    public async Task ReleaseAsync(CancellationToken ct)
    {
        // Idempotent — the adapter's delete is already a no-op on 404.
        await this.client.DeleteDocumentAsync(
            CosmosContainerNames.Leases,
            LeaseDocumentId,
            LeaseDocumentId,
            ct).ConfigureAwait(false);
    }

    private static bool TryParseExpiry(string value, out DateTimeOffset expiresAt)
    {
        return DateTimeOffset.TryParse(
            value,
            CultureInfo.InvariantCulture,
            DateTimeStyles.RoundtripKind,
            out expiresAt);
    }
}
