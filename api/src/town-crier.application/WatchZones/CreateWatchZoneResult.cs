using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.WatchZones;

public sealed record CreateWatchZoneResult(IReadOnlyCollection<PlanningApplication> NearbyApplications);
