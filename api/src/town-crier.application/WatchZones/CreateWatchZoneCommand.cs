namespace TownCrier.Application.WatchZones;

public sealed record CreateWatchZoneCommand(
    string UserId,
    string ZoneId,
    string Name,
    double Latitude,
    double Longitude,
    double RadiusMetres,
    int? AuthorityId = null);
