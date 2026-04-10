using TownCrier.Domain.Geocoding;

namespace TownCrier.Domain.WatchZones;

public sealed class WatchZone
{
    public WatchZone(string id, string userId, string name, Coordinates centre, double radiusMetres, int authorityId, DateTimeOffset createdAt)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(id);
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);
        ArgumentException.ThrowIfNullOrWhiteSpace(name);
        ArgumentNullException.ThrowIfNull(centre);
        ArgumentOutOfRangeException.ThrowIfNegativeOrZero(radiusMetres);
        ArgumentOutOfRangeException.ThrowIfNegativeOrZero(authorityId);

        this.Id = id;
        this.UserId = userId;
        this.Name = name;
        this.Centre = centre;
        this.RadiusMetres = radiusMetres;
        this.AuthorityId = authorityId;
        this.CreatedAt = createdAt;
    }

    public string Id { get; }

    public string UserId { get; }

    public string Name { get; }

    public Coordinates Centre { get; }

    public double RadiusMetres { get; }

    public int AuthorityId { get; }

    public DateTimeOffset CreatedAt { get; }
}
