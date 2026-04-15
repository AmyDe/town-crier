using TownCrier.Application.UserProfiles;

namespace TownCrier.Application.Admin;

public sealed class ListUsersQueryHandler
{
    private readonly IUserProfileRepository repository;

    public ListUsersQueryHandler(IUserProfileRepository repository)
    {
        this.repository = repository;
    }

    public async Task<ListUsersResult> HandleAsync(ListUsersQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        var page = await this.repository.ListAsync(
            query.SearchTerm,
            query.PageSize,
            query.ContinuationToken,
            ct).ConfigureAwait(false);

        var items = page.Profiles
            .Select(p => new ListUsersItem(p.UserId, p.Email, p.Tier))
            .ToList();

        return new ListUsersResult(items, page.ContinuationToken);
    }
}
