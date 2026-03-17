namespace TownCrier.Application.SavedApplications;

public sealed class GetSavedApplicationsQueryHandler
{
    private readonly ISavedApplicationRepository repository;

    public GetSavedApplicationsQueryHandler(ISavedApplicationRepository repository)
    {
        this.repository = repository;
    }

    public async Task<IReadOnlyList<SavedApplicationResult>> HandleAsync(GetSavedApplicationsQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        var saved = await this.repository.GetByUserIdAsync(query.UserId, ct).ConfigureAwait(false);

        return saved.Select(s => new SavedApplicationResult(s.ApplicationUid, s.SavedAt)).ToList();
    }
}
