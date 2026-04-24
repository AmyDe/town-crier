namespace TownCrier.Application.Polling;

/// <summary>
/// Opaque token returned by a successful <see cref="IPollingLeaseStore.TryAcquireAsync"/>.
/// Carries the ETag of the winning write so <see cref="IPollingLeaseStore.ReleaseAsync"/>
/// can perform a conditional delete.
/// </summary>
public sealed record LeaseHandle(string ETag);
