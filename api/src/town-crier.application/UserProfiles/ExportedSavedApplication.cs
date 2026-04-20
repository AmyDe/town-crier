namespace TownCrier.Application.UserProfiles;

public sealed record ExportedSavedApplication(
    string ApplicationUid,
    DateTimeOffset SavedAt);
