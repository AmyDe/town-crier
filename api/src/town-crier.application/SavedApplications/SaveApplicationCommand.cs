using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.SavedApplications;

public sealed record SaveApplicationCommand(string UserId, PlanningApplication Application);
