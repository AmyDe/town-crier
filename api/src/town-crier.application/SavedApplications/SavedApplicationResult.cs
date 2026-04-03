using TownCrier.Application.PlanningApplications;

namespace TownCrier.Application.SavedApplications;

public sealed record SavedApplicationResult(
    string ApplicationUid,
    DateTimeOffset SavedAt,
    PlanningApplicationResult Application);
