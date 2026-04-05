namespace TownCrier.Application.UserProfiles;

public sealed class GetUserProfileQueryHandler
{
    private readonly IUserProfileRepository repository;

    public GetUserProfileQueryHandler(IUserProfileRepository repository)
    {
        this.repository = repository;
    }

    public async Task<GetUserProfileResult?> HandleAsync(GetUserProfileQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        var profile = await this.repository.GetByUserIdAsync(query.UserId, ct).ConfigureAwait(false);
        if (profile is null)
        {
            return null;
        }

        return new GetUserProfileResult(
            profile.UserId,
            profile.NotificationPreferences.PushEnabled,
            profile.Tier);
    }
}
