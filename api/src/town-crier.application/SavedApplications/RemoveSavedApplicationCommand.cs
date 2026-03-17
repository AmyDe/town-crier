namespace TownCrier.Application.SavedApplications;

public sealed record RemoveSavedApplicationCommand(string UserId, string ApplicationUid);
