namespace TownCrier.Application.WatchZones;

public sealed record UpdateWatchZoneCommand(
    string UserId,
    string ZoneId,
    string? Name = null,
    double? Latitude = null,
    double? Longitude = null,
    double? RadiusMetres = null,
    int? AuthorityId = null,
    bool? PushEnabled = null,
    bool? EmailInstantEnabled = null);
