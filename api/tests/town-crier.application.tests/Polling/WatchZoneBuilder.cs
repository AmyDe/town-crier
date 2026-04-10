using TownCrier.Domain.Geocoding;
using TownCrier.Domain.WatchZones;

namespace TownCrier.Application.Tests.Polling;

internal sealed class WatchZoneBuilder
{
    private string id = "zone-1";
    private string userId = "user-1";
    private string name = "Default Zone";
    private double latitude = 51.5074;
    private double longitude = -0.1278;
    private double radiusMetres = 5000;
    private int authorityId = 1;
    private DateTimeOffset createdAt = new(2026, 1, 1, 0, 0, 0, TimeSpan.Zero);

    public WatchZoneBuilder WithId(string id)
    {
        this.id = id;
        return this;
    }

    public WatchZoneBuilder WithUserId(string userId)
    {
        this.userId = userId;
        return this;
    }

    public WatchZoneBuilder WithName(string name)
    {
        this.name = name;
        return this;
    }

    public WatchZoneBuilder WithAuthorityId(int authorityId)
    {
        this.authorityId = authorityId;
        return this;
    }

    public WatchZoneBuilder WithCentre(double latitude, double longitude)
    {
        this.latitude = latitude;
        this.longitude = longitude;
        return this;
    }

    public WatchZoneBuilder WithRadiusMetres(double radiusMetres)
    {
        this.radiusMetres = radiusMetres;
        return this;
    }

    public WatchZoneBuilder WithCreatedAt(DateTimeOffset createdAt)
    {
        this.createdAt = createdAt;
        return this;
    }

    public WatchZone Build()
    {
        return new WatchZone(
            this.id,
            this.userId,
            this.name,
            new Coordinates(this.latitude, this.longitude),
            this.radiusMetres,
            this.authorityId,
            this.createdAt);
    }
}
