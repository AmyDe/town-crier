using TownCrier.Application.Notifications;
using TownCrier.Domain.NotificationState;

namespace TownCrier.Application.NotificationState;

/// <summary>
/// Loads the caller's <see cref="NotificationStateAggregate"/> and the unread
/// notification count derived from it. First-touch users (no document yet) get
/// a fresh watermark seeded at the current instant and persisted, so all of
/// their existing notifications count as already read — see spec Pre-Resolved
/// Decision #13 ("clean slate at deploy time").
/// </summary>
public sealed class GetNotificationStateQueryHandler
{
    private readonly INotificationStateRepository stateRepository;
    private readonly INotificationRepository notificationRepository;
    private readonly TimeProvider timeProvider;

    public GetNotificationStateQueryHandler(
        INotificationStateRepository stateRepository,
        INotificationRepository notificationRepository,
        TimeProvider timeProvider)
    {
        ArgumentNullException.ThrowIfNull(stateRepository);
        ArgumentNullException.ThrowIfNull(notificationRepository);
        ArgumentNullException.ThrowIfNull(timeProvider);

        this.stateRepository = stateRepository;
        this.notificationRepository = notificationRepository;
        this.timeProvider = timeProvider;
    }

    public async Task<GetNotificationStateResult> HandleAsync(
        GetNotificationStateQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        var state = await this.stateRepository
            .GetByUserIdAsync(query.UserId, ct).ConfigureAwait(false);

        if (state is null)
        {
            // First-touch: seed at now and persist so subsequent reads are
            // idempotent. The unread count is then by definition zero.
            state = NotificationStateAggregate.Create(
                query.UserId, this.timeProvider.GetUtcNow());
            await this.stateRepository.SaveAsync(state, ct).ConfigureAwait(false);
        }

        var unreadCount = await this.notificationRepository
            .GetUnreadCountAsync(query.UserId, state.LastReadAt, ct)
            .ConfigureAwait(false);

        return new GetNotificationStateResult(
            state.LastReadAt, state.Version, unreadCount);
    }
}
