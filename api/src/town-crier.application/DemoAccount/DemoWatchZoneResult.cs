namespace TownCrier.Application.DemoAccount;

public sealed record DemoWatchZoneResult(
    string ZoneId,
    string AuthorityName,
    double Latitude,
    double Longitude,
    double RadiusMetres);
