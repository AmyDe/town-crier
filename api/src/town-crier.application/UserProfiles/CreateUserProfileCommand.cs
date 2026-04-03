namespace TownCrier.Application.UserProfiles;

public sealed record CreateUserProfileCommand(string UserId, string? Email = null, bool EmailVerified = false);
