using TownCrier.Domain.Geocoding;
using TownCrier.Domain.Groups;

namespace TownCrier.Infrastructure.Groups;

internal sealed class GroupDocument
{
    public required string Id { get; init; }

    public required string Type { get; init; }

    public required string OwnerId { get; init; }

    public required string Name { get; init; }

    public required double CentreLatitude { get; init; }

    public required double CentreLongitude { get; init; }

    public required double RadiusMetres { get; init; }

    public required int AuthorityId { get; init; }

    public required DateTimeOffset CreatedAt { get; init; }

    public required List<GroupMemberDocument> Members { get; init; }

    public static GroupDocument FromDomain(Group group)
    {
        ArgumentNullException.ThrowIfNull(group);

        return new GroupDocument
        {
            Id = group.Id,
            Type = "group",
            OwnerId = group.OwnerId,
            Name = group.Name,
            CentreLatitude = group.Centre.Latitude,
            CentreLongitude = group.Centre.Longitude,
            RadiusMetres = group.RadiusMetres,
            AuthorityId = group.AuthorityId,
            CreatedAt = group.CreatedAt,
            Members = group.Members.Select(GroupMemberDocument.FromDomain).ToList(),
        };
    }

    public Group ToDomain()
    {
        var members = this.Members.Select(m => m.ToDomain());

        return Group.Reconstitute(
            this.Id,
            this.Name,
            this.OwnerId,
            new Coordinates(this.CentreLatitude, this.CentreLongitude),
            this.RadiusMetres,
            this.AuthorityId,
            this.CreatedAt,
            members);
    }
}
