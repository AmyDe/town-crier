namespace TownCrier.Application.Admin;

public sealed record ListUsersResult(IReadOnlyList<ListUsersItem> Items, string? ContinuationToken);
