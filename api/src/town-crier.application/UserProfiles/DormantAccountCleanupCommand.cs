namespace TownCrier.Application.UserProfiles;

/// <summary>
/// Command to delete profiles whose LastActiveAt precedes <see cref="Now"/> minus
/// the retention window (12 months). Enforces UK GDPR Art. 5(1)(e) storage
/// limitation — the privacy policy commits to deleting accounts after 12 months
/// of inactivity.
/// </summary>
public sealed record DormantAccountCleanupCommand(DateTimeOffset Now);
