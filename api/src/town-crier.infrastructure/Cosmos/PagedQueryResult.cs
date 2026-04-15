namespace TownCrier.Infrastructure.Cosmos;

public sealed record PagedQueryResult<T>(IReadOnlyList<T> Items, string? ContinuationToken);
