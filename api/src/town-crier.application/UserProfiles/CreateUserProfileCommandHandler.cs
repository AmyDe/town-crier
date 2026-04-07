using TownCrier.Application.Observability;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.UserProfiles;

public sealed class CreateUserProfileCommandHandler
{
    private static readonly DateTimeOffset FarFutureExpiry = new(2099, 12, 31, 0, 0, 0, TimeSpan.Zero);

    private readonly IUserProfileRepository repository;
    private readonly AutoGrantOptions autoGrantOptions;

    public CreateUserProfileCommandHandler(IUserProfileRepository repository, AutoGrantOptions autoGrantOptions)
    {
        this.repository = repository;
        this.autoGrantOptions = autoGrantOptions;
    }

    public async Task<CreateUserProfileResult> HandleAsync(CreateUserProfileCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var existing = await this.repository.GetByUserIdAsync(command.UserId, ct).ConfigureAwait(false);
        if (existing is not null)
        {
            if (existing.Email is null && !string.IsNullOrWhiteSpace(command.Email))
            {
                existing.BackfillEmail(command.Email);
                await this.repository.SaveAsync(existing, ct).ConfigureAwait(false);
            }

            return new CreateUserProfileResult(
                existing.UserId,
                existing.NotificationPreferences.PushEnabled,
                existing.Tier);
        }

        var profile = UserProfile.Register(command.UserId, command.Email);

        if (command.EmailVerified && this.autoGrantOptions.IsProDomain(command.Email))
        {
            profile.ActivateSubscription(SubscriptionTier.Pro, FarFutureExpiry);
        }

        await this.repository.SaveAsync(profile, ct).ConfigureAwait(false);
        ApiMetrics.UsersRegistered.Add(1);

        return new CreateUserProfileResult(
            profile.UserId,
            profile.NotificationPreferences.PushEnabled,
            profile.Tier);
    }
}
