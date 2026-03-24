namespace TownCrier.Application.WatchZones;

public sealed record ListWatchZonesResult(IReadOnlyCollection<WatchZoneSummary> Zones);
