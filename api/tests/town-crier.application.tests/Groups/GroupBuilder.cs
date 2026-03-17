using TownCrier.Domain.Geocoding;
using TownCrier.Domain.Groups;

namespace TownCrier.Application.Tests.Groups;

internal sealed class GroupBuilder
{
    private string id = "group-1";
    private string name = "Test Group";
    private string ownerId = "user-1";
    private double latitude = 51.5074;
    private double longitude = -0.1278;
    private double radiusMetres = 5000;
    private int authorityId = 1;
    private DateTimeOffset createdAt = new(2026, 3, 17, 10, 0, 0, TimeSpan.Zero);

    public GroupBuilder WithId(string id)
    {
        this.id = id;
        return this;
    }

    public GroupBuilder WithName(string name)
    {
        this.name = name;
        return this;
    }

    public GroupBuilder WithOwnerId(string ownerId)
    {
        this.ownerId = ownerId;
        return this;
    }

    public GroupBuilder WithCentre(double latitude, double longitude)
    {
        this.latitude = latitude;
        this.longitude = longitude;
        return this;
    }

    public GroupBuilder WithRadiusMetres(double radiusMetres)
    {
        this.radiusMetres = radiusMetres;
        return this;
    }

    public GroupBuilder WithAuthorityId(int authorityId)
    {
        this.authorityId = authorityId;
        return this;
    }

    public GroupBuilder WithCreatedAt(DateTimeOffset createdAt)
    {
        this.createdAt = createdAt;
        return this;
    }

    public Group Build()
    {
        return Group.Create(
            this.id,
            this.name,
            this.ownerId,
            new Coordinates(this.latitude, this.longitude),
            this.radiusMetres,
            this.authorityId,
            this.createdAt);
    }
}
