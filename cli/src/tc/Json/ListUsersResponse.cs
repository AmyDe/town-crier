namespace Tc.Json;

internal sealed class ListUsersResponse
{
    public required IReadOnlyList<ListUsersItemResponse> Items { get; init; }

    public string? ContinuationToken { get; init; }
}
