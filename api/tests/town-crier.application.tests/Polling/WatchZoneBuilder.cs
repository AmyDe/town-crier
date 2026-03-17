using TownCrier.Domain.Geocoding;
using TownCrier.Domain.WatchZones;

namespace TownCrier.Application.Tests.Polling;

internal sealed class WatchZoneBuilder
{
    private string id = "zone-1";
    private string userId = "user-1";
    private double latitude = 51.5074;
    private double longitude = -0.1278;
    private double radiusMetres = 5000;
    private int authorityId = 1;

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

    public WatchZone Build()
    {
        return new WatchZone(
            this.id,
            this.userId,
            new Coordinates(this.latitude, this.longitude),
            this.radiusMetres,
            this.authorityId);
    }
}
