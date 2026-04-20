namespace TownCrier.Application.UserProfiles;

/// <summary>
/// Marks the given user as active at the supplied timestamp. Invoked on every
/// authenticated API request so the dormant-account cleanup job can identify
/// accounts that have been inactive for >= 12 months (UK GDPR Art. 5(1)(e)).
/// </summary>
public sealed record RecordUserActivityCommand(string UserId, DateTimeOffset Now);
