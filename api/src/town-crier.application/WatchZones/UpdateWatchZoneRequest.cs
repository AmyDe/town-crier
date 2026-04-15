namespace TownCrier.Application.WatchZones;

public sealed record UpdateWatchZoneRequest(
    string? Name = null,
    double? Latitude = null,
    double? Longitude = null,
    double? RadiusMetres = null,
    int? AuthorityId = null);
