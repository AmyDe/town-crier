namespace TownCrier.Application.UserProfiles;

public sealed record UpdateUserProfileCommand(
    string UserId,
    bool PushEnabled,
    DayOfWeek DigestDay = DayOfWeek.Monday,
    bool EmailDigestEnabled = true,
    bool EmailInstantEnabled = false);
