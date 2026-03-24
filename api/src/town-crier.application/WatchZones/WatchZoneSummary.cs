namespace TownCrier.Application.WatchZones;

public sealed record WatchZoneSummary(
    string Id,
    string Name,
    double Latitude,
    double Longitude,
    double RadiusMetres,
    int AuthorityId);
