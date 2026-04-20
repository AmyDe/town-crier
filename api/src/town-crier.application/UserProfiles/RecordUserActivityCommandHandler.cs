namespace TownCrier.Application.UserProfiles;

/// <summary>
/// Updates the user's LastActiveAt timestamp. Called from request middleware on
/// every authenticated API call. Skips the Cosmos write if the profile was
/// already marked active within the past 24 hours so the endpoint does not
/// incur an upsert per request.
/// </summary>
public sealed class RecordUserActivityCommandHandler
{
    private static readonly TimeSpan WriteDedupeWindow = TimeSpan.FromHours(24);

    private readonly IUserProfileRepository repository;

    public RecordUserActivityCommandHandler(IUserProfileRepository repository)
    {
        this.repository = repository;
    }

    public async Task HandleAsync(RecordUserActivityCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var profile = await this.repository.GetByUserIdAsync(command.UserId, ct).ConfigureAwait(false);
        if (profile is null)
        {
            // Unknown or deleted user — nothing to record. The middleware must not
            // auto-create profiles; registration happens via POST /v1/me.
            return;
        }

        if (command.Now - profile.LastActiveAt < WriteDedupeWindow)
        {
            return;
        }

        profile.RecordActivity(command.Now);
        await this.repository.SaveAsync(profile, ct).ConfigureAwait(false);
    }
}
