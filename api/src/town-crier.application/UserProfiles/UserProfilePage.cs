using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.UserProfiles;

public sealed record UserProfilePage(IReadOnlyList<UserProfile> Profiles, string? ContinuationToken);
