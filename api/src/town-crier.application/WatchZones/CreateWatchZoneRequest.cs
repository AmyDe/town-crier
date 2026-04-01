namespace TownCrier.Application.WatchZones;

public sealed record CreateWatchZoneRequest(
    string Name,
    double Latitude,
    double Longitude,
    double RadiusMetres,
    int? AuthorityId = null);
