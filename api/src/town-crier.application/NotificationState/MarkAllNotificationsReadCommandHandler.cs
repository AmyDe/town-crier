using TownCrier.Domain.NotificationState;

namespace TownCrier.Application.NotificationState;

/// <summary>
/// Handles the user-driven Mark-All-Read action by stamping the watermark to
/// the current instant. First-touch users (no aggregate yet) get one created
/// at the same instant — the end-state is identical either way.
/// </summary>
public sealed class MarkAllNotificationsReadCommandHandler
{
    private readonly INotificationStateRepository repository;
    private readonly TimeProvider timeProvider;

    public MarkAllNotificationsReadCommandHandler(
        INotificationStateRepository repository,
        TimeProvider timeProvider)
    {
        ArgumentNullException.ThrowIfNull(repository);
        ArgumentNullException.ThrowIfNull(timeProvider);

        this.repository = repository;
        this.timeProvider = timeProvider;
    }

    public async Task HandleAsync(
        MarkAllNotificationsReadCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var now = this.timeProvider.GetUtcNow();
        var state = await this.repository
            .GetByUserIdAsync(command.UserId, ct).ConfigureAwait(false);

        if (state is null)
        {
            // First-touch path: a fresh aggregate seeded at now is the same
            // end-state as create-then-mark. Skip the redundant version bump.
            state = NotificationStateAggregate.Create(command.UserId, now);
        }
        else
        {
            state.MarkAllReadAt(now);
        }

        await this.repository.SaveAsync(state, ct).ConfigureAwait(false);
    }
}
