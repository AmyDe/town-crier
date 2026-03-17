namespace TownCrier.Application.Groups;

public sealed record CreateGroupCommand(
    string UserId,
    string GroupId,
    string Name,
    double Latitude,
    double Longitude,
    double RadiusMetres,
    int AuthorityId);
