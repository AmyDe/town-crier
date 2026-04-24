using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Polling;

/// <summary>
/// Cosmos-persisted document for the polling lease. The Leases container is
/// partitioned by <c>/id</c>, so each lease (currently only <c>"polling"</c>)
/// lives in its own logical partition.
/// </summary>
internal sealed class PollingLeaseDocument
{
    [JsonPropertyName("id")]
    public string Id { get; set; } = string.Empty;

    /// <summary>
    /// Gets or sets the stable holder identifier (machine name / random GUID).
    /// Diagnostic only — acquisition decisions are based purely on
    /// <see cref="ExpiresAtUtc"/>.
    /// </summary>
    [JsonPropertyName("holderId")]
    public string HolderId { get; set; } = string.Empty;

    /// <summary>
    /// Gets or sets the UTC instant the lease was acquired. ISO-8601 round-trip
    /// ("o") string. Diagnostic only — acquisition decisions are based purely on
    /// <see cref="ExpiresAtUtc"/>.
    /// </summary>
    [JsonPropertyName("acquiredAtUtc")]
    public string AcquiredAtUtc { get; set; } = string.Empty;

    /// <summary>
    /// Gets or sets the UTC instant after which the lease is considered expired.
    /// Stored as an ISO-8601 round-trip ("o") string for human readability in the
    /// Data Explorer.
    /// </summary>
    [JsonPropertyName("expiresAtUtc")]
    public string ExpiresAtUtc { get; set; } = string.Empty;
}
