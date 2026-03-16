namespace TownCrier.Application.UserProfiles;

public sealed record UpdateUserProfileCommand(
    string UserId,
    string? Postcode,
    bool PushEnabled);
