using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.Auth;
using TownCrier.Application.Observability;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.UserProfiles;

public sealed class CreateUserProfileCommandHandler
{
    private static readonly DateTimeOffset FarFutureExpiry = new(2099, 12, 31, 0, 0, 0, TimeSpan.Zero);

    private static readonly Action<ILogger, string, string, string, Exception?> LogTierDriftDetected =
        LoggerMessage.Define<string, string, string>(
            LogLevel.Warning,
            new EventId(1, nameof(LogTierDriftDetected)),
            "Auth0 metadata tier drift detected for user {UserId}: Cosmos={CosmosTier}, JWT={JwtTier}. Backfilling Auth0.");

    private static readonly Action<ILogger, string, Exception?> LogTierBackfillFailed =
        LoggerMessage.Define<string>(
            LogLevel.Warning,
            new EventId(2, nameof(LogTierBackfillFailed)),
            "Auth0 tier backfill failed for user {UserId}; will retry on next POST /v1/me.");

    private readonly IUserProfileRepository repository;
    private readonly AutoGrantOptions autoGrantOptions;
    private readonly IAuth0ManagementClient auth0Client;
    private readonly ILogger<CreateUserProfileCommandHandler> logger;

    public CreateUserProfileCommandHandler(
        IUserProfileRepository repository,
        AutoGrantOptions autoGrantOptions,
        IAuth0ManagementClient auth0Client)
        : this(repository, autoGrantOptions, auth0Client, NullLogger<CreateUserProfileCommandHandler>.Instance)
    {
    }

    public CreateUserProfileCommandHandler(
        IUserProfileRepository repository,
        AutoGrantOptions autoGrantOptions,
        IAuth0ManagementClient auth0Client,
        ILogger<CreateUserProfileCommandHandler> logger)
    {
        this.repository = repository;
        this.autoGrantOptions = autoGrantOptions;
        this.auth0Client = auth0Client;
        this.logger = logger;
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

            await this.TryBackfillAuth0TierAsync(existing, command.JwtSubscriptionTier, ct).ConfigureAwait(false);

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

    private async Task TryBackfillAuth0TierAsync(UserProfile profile, string? jwtSubscriptionTier, CancellationToken ct)
    {
        if (string.IsNullOrWhiteSpace(jwtSubscriptionTier))
        {
            return;
        }

        var cosmosTier = profile.Tier.ToString();
        if (string.Equals(cosmosTier, jwtSubscriptionTier, StringComparison.OrdinalIgnoreCase))
        {
            return;
        }

        LogTierDriftDetected(this.logger, profile.UserId, cosmosTier, jwtSubscriptionTier, null);

        try
        {
            await this.auth0Client.UpdateSubscriptionTierAsync(profile.UserId, cosmosTier, ct).ConfigureAwait(false);
        }
        catch (OperationCanceledException)
        {
            throw;
        }
#pragma warning disable CA1031 // Do not catch general exception types
        catch (Exception ex)
#pragma warning restore CA1031
        {
            // Best-effort backfill: an Auth0 outage must never fail POST /v1/me.
            // Next POST /v1/me will retry.
            LogTierBackfillFailed(this.logger, profile.UserId, ex);
        }
    }
}
