namespace TownCrier.Application.Search;

public sealed record SearchPlanningApplicationsQuery(
    string UserId,
    string SearchText,
    int AuthorityId,
    int Page = 1);
