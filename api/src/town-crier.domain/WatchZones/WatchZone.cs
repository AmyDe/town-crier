using TownCrier.Domain.Geocoding;

namespace TownCrier.Domain.WatchZones;

public sealed class WatchZone
{
    public WatchZone(string id, string userId, Coordinates centre, double radiusMetres, int authorityId)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(id);
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);
        ArgumentNullException.ThrowIfNull(centre);
        ArgumentOutOfRangeException.ThrowIfNegativeOrZero(radiusMetres);
        ArgumentOutOfRangeException.ThrowIfNegativeOrZero(authorityId);

        this.Id = id;
        this.UserId = userId;
        this.Centre = centre;
        this.RadiusMetres = radiusMetres;
        this.AuthorityId = authorityId;
    }

    public string Id { get; }

    public string UserId { get; }

    public Coordinates Centre { get; }

    public double RadiusMetres { get; }

    public int AuthorityId { get; }
}
