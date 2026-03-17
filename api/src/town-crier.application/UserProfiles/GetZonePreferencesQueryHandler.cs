namespace TownCrier.Application.UserProfiles;

public sealed class GetZonePreferencesQueryHandler
{
    private readonly IUserProfileRepository repository;

    public GetZonePreferencesQueryHandler(IUserProfileRepository repository)
    {
        this.repository = repository;
    }

    public async Task<GetZonePreferencesResult> HandleAsync(
        GetZonePreferencesQuery query,
        CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        var profile = await this.repository.GetByUserIdAsync(query.UserId, ct).ConfigureAwait(false)
            ?? throw UserProfileNotFoundException.ForUser(query.UserId);

        var prefs = profile.GetZonePreferences(query.ZoneId);

        return new GetZonePreferencesResult(
            query.ZoneId,
            prefs.NewApplications,
            prefs.StatusChanges,
            prefs.DecisionUpdates);
    }
}
