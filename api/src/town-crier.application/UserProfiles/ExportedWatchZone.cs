namespace TownCrier.Application.UserProfiles;

public sealed record ExportedWatchZone(
    string Id,
    string Name,
    double Latitude,
    double Longitude,
    double RadiusMetres,
    int AuthorityId,
    DateTimeOffset CreatedAt);
