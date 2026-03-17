namespace TownCrier.Application.UserProfiles;

public sealed class ExportUserDataQueryHandler
{
    private readonly IUserProfileRepository repository;

    public ExportUserDataQueryHandler(IUserProfileRepository repository)
    {
        this.repository = repository;
    }

    public async Task<ExportUserDataResult?> HandleAsync(ExportUserDataQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);
        var profile = await this.repository.GetByUserIdAsync(query.UserId, ct).ConfigureAwait(false);
        if (profile is null)
        {
            return null;
        }

        return new ExportUserDataResult(
            profile.UserId,
            profile.Postcode,
            profile.NotificationPreferences.PushEnabled,
            profile.Tier);
    }
}
