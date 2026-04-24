using System.Globalization;
using TownCrier.Application.Polling;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Polling;

/// <summary>
/// ETag-CAS-backed <see cref="IPollingLeaseStore"/> over the Cosmos
/// <c>Leases</c> container. A single document with id <c>"polling"</c> gates
/// concurrent poll cycles between the Service Bus-triggered orchestrator and
/// the safety-net bootstrap cron.
///
/// Acquisition is a true compare-and-swap: a missing document is created with
/// <c>If-None-Match: *</c>; an expired document is replaced with
/// <c>If-Match: &lt;etag&gt;</c>. Cosmos returns <c>409</c> / <c>412</c> when a
/// concurrent writer wins the race, which the store maps to
/// <see cref="LeaseAcquireResult.Held"/>. Transient (5xx / network) failures are
/// surfaced as <see cref="LeaseAcquireResult.TransientError"/>. See
/// <c>docs/specs/polling-lease-cas.md</c>.
/// </summary>
public sealed class CosmosPollingLeaseStore : IPollingLeaseStore
{
    private const string LeaseDocumentId = "polling";

    private readonly ICosmosRestClient client;
    private readonly TimeProvider timeProvider;
    private readonly string holderId;

    /// <summary>
    /// Initializes a new instance of the <see cref="CosmosPollingLeaseStore"/> class.
    /// </summary>
    /// <param name="client">Cosmos REST client used for CAS-aware reads, creates, replaces and deletes against the Leases container.</param>
    /// <param name="timeProvider">Clock used to compute <c>expiresAtUtc</c> on acquire and to evaluate liveness of an existing lease.</param>
    public CosmosPollingLeaseStore(ICosmosRestClient client, TimeProvider timeProvider)
    {
        ArgumentNullException.ThrowIfNull(client);
        ArgumentNullException.ThrowIfNull(timeProvider);

        this.client = client;
        this.timeProvider = timeProvider;

        // One holderId per process instance. Written as diagnostic metadata; lease
        // decisions compare ExpiresAtUtc + ETag, not holder identity.
        this.holderId = Guid.NewGuid().ToString("N");
    }

    /// <inheritdoc />
    public async Task<LeaseAcquireResult> TryAcquireAsync(TimeSpan ttl, CancellationToken ct)
    {
        try
        {
            var now = this.timeProvider.GetUtcNow();
            var read = await this.client.ReadDocumentWithETagAsync(
                CosmosContainerNames.Leases,
                LeaseDocumentId,
                LeaseDocumentId,
                CosmosJsonSerializerContext.Default.PollingLeaseDocument,
                ct).ConfigureAwait(false);

            var desired = new PollingLeaseDocument
            {
                Id = LeaseDocumentId,
                HolderId = this.holderId,
                AcquiredAtUtc = now.ToString("o", CultureInfo.InvariantCulture),
                ExpiresAtUtc = (now + ttl).ToString("o", CultureInfo.InvariantCulture),
            };

            if (read.Document is null)
            {
                var created = await this.client.TryCreateDocumentAsync(
                    CosmosContainerNames.Leases,
                    desired,
                    LeaseDocumentId,
                    CosmosJsonSerializerContext.Default.PollingLeaseDocument,
                    ct).ConfigureAwait(false);
                if (!created)
                {
                    return LeaseAcquireResult.FromHeld();
                }

                // Read back to capture the server-assigned ETag — TryCreateDocumentAsync
                // returns bool only, so the caller has to round-trip to learn the new ETag
                // needed for a CAS release.
                var afterCreate = await this.client.ReadDocumentWithETagAsync(
                    CosmosContainerNames.Leases,
                    LeaseDocumentId,
                    LeaseDocumentId,
                    CosmosJsonSerializerContext.Default.PollingLeaseDocument,
                    ct).ConfigureAwait(false);
                return afterCreate.ETag is null
                    ? LeaseAcquireResult.FromHeld()
                    : LeaseAcquireResult.FromAcquired(new LeaseHandle(afterCreate.ETag));
            }

            if (TryParseExpiry(read.Document.ExpiresAtUtc, out var expiresAt) && expiresAt > now)
            {
                return LeaseAcquireResult.FromHeld();
            }

            var replaced = await this.client.TryReplaceDocumentAsync(
                CosmosContainerNames.Leases,
                desired,
                LeaseDocumentId,
                read.ETag!,
                CosmosJsonSerializerContext.Default.PollingLeaseDocument,
                ct).ConfigureAwait(false);
            if (!replaced)
            {
                return LeaseAcquireResult.FromHeld();
            }

            var afterReplace = await this.client.ReadDocumentWithETagAsync(
                CosmosContainerNames.Leases,
                LeaseDocumentId,
                LeaseDocumentId,
                CosmosJsonSerializerContext.Default.PollingLeaseDocument,
                ct).ConfigureAwait(false);
            return afterReplace.ETag is null
                ? LeaseAcquireResult.FromHeld()
                : LeaseAcquireResult.FromAcquired(new LeaseHandle(afterReplace.ETag));
        }
#pragma warning disable CA1031 // Convert transient failures to a result type; swallow above classes of exception
        catch (Exception ex)
#pragma warning restore CA1031
        {
            return LeaseAcquireResult.FromTransient(ex);
        }
    }

    /// <inheritdoc />
    public async Task ReleaseAsync(LeaseHandle handle, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(handle);
        try
        {
            _ = await this.client.TryDeleteDocumentAsync(
                CosmosContainerNames.Leases,
                LeaseDocumentId,
                LeaseDocumentId,
                handle.ETag,
                ct).ConfigureAwait(false);

            // Caller (orchestrator / bootstrap) logs the outcome via its own logger.
        }
#pragma warning disable CA1031 // Release is best-effort; TTL is the backstop.
        catch
#pragma warning restore CA1031
        {
            // Swallow; TTL is the backstop.
        }
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
