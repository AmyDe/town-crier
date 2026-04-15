namespace TownCrier.Application.Admin;

public sealed record ListUsersQuery(string? SearchTerm, int PageSize, string? ContinuationToken);
