namespace Tc.Json;

internal sealed class ListUsersItemResponse
{
    public required string UserId { get; init; }

    public string? Email { get; init; }

    public required string Tier { get; init; }
}
