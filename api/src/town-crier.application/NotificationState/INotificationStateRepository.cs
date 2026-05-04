using TownCrier.Domain.NotificationState;

namespace TownCrier.Application.NotificationState;

/// <summary>
/// Port for persisting per-user notification watermarks. Implementations are
/// expected to be partition-key-isolated by userId — there is one document per
/// user, with the userId as both the document id and partition key.
/// </summary>
public interface INotificationStateRepository
{
    /// <summary>
    /// Loads the watermark aggregate for the given user, or returns <c>null</c>
    /// if the user has no document yet (first-touch path; the endpoint adapter
    /// then seeds and saves).
    /// </summary>
    /// <param name="userId">The Auth0 sub for the owning user.</param>
    /// <param name="ct">Cancellation token.</param>
    /// <returns>The hydrated aggregate, or <c>null</c> when no state exists.</returns>
    Task<NotificationStateAggregate?> GetByUserIdAsync(string userId, CancellationToken ct);

    /// <summary>
    /// Persists the aggregate. Upsert semantics: a missing document is created,
    /// an existing one is replaced wholesale.
    /// </summary>
    /// <param name="state">The aggregate to persist.</param>
    /// <param name="ct">Cancellation token.</param>
    Task SaveAsync(NotificationStateAggregate state, CancellationToken ct);
}
