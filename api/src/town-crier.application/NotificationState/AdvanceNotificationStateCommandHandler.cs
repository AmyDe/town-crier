using TownCrier.Domain.NotificationState;

namespace TownCrier.Application.NotificationState;

/// <summary>
/// Advances the caller's watermark to <c>command.AsOf</c> only if it lies
/// strictly after the current value (monotonic — see
/// <see cref="NotificationStateAggregate.AdvanceTo"/>). First-touch users get
/// a fresh aggregate seeded at <see cref="TimeProvider.GetUtcNow"/> first, and
/// the advance is then applied normally; if asOf is already at or before the
/// seed instant, only the seed is persisted.
/// </summary>
public sealed class AdvanceNotificationStateCommandHandler
{
    private readonly INotificationStateRepository repository;
    private readonly TimeProvider timeProvider;

    public AdvanceNotificationStateCommandHandler(
        INotificationStateRepository repository,
        TimeProvider timeProvider)
    {
        ArgumentNullException.ThrowIfNull(repository);
        ArgumentNullException.ThrowIfNull(timeProvider);

        this.repository = repository;
        this.timeProvider = timeProvider;
    }

    public async Task HandleAsync(
        AdvanceNotificationStateCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var state = await this.repository
            .GetByUserIdAsync(command.UserId, ct).ConfigureAwait(false);

        if (state is null)
        {
            // First-touch: seed at now so the user has a baseline, then attempt
            // the advance. Persist regardless so a subsequent GET sees the seed
            // even when asOf was a no-op against it.
            state = NotificationStateAggregate.Create(
                command.UserId, this.timeProvider.GetUtcNow());
            state.AdvanceTo(command.AsOf);
            await this.repository.SaveAsync(state, ct).ConfigureAwait(false);
            return;
        }

        // Existing state: avoid a redundant write when asOf is stale. AdvanceTo
        // returns false in that case and leaves the aggregate untouched.
        if (!state.AdvanceTo(command.AsOf))
        {
            return;
        }

        await this.repository.SaveAsync(state, ct).ConfigureAwait(false);
    }
}
