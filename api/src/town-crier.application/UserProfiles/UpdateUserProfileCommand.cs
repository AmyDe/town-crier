namespace TownCrier.Application.UserProfiles;

public sealed record UpdateUserProfileCommand(
    string UserId,
    bool PushEnabled);
