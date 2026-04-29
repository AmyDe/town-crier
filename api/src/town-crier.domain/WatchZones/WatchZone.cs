using TownCrier.Domain.Geocoding;

namespace TownCrier.Domain.WatchZones;

public sealed class WatchZone
{
    public WatchZone(
        string id,
        string userId,
        string name,
        Coordinates centre,
        double radiusMetres,
        int authorityId,
        DateTimeOffset createdAt,
        bool pushEnabled = true,
        bool emailInstantEnabled = true)
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
        this.PushEnabled = pushEnabled;
        this.EmailInstantEnabled = emailInstantEnabled;
    }

    public string Id { get; }

    public string UserId { get; }

    public string Name { get; }

    public Coordinates Centre { get; }

    public double RadiusMetres { get; }

    public int AuthorityId { get; }

    public DateTimeOffset CreatedAt { get; }

    public bool PushEnabled { get; }

    public bool EmailInstantEnabled { get; }

    public WatchZone WithUpdates(
        string? name = null,
        Coordinates? centre = null,
        double? radiusMetres = null,
        int? authorityId = null,
        bool? pushEnabled = null,
        bool? emailInstantEnabled = null)
    {
        var newName = name ?? this.Name;
        var newCentre = centre ?? this.Centre;
        var newRadius = radiusMetres ?? this.RadiusMetres;
        var newAuthorityId = authorityId ?? this.AuthorityId;
        var newPushEnabled = pushEnabled ?? this.PushEnabled;
        var newEmailInstantEnabled = emailInstantEnabled ?? this.EmailInstantEnabled;

        ArgumentException.ThrowIfNullOrWhiteSpace(newName);
        ArgumentOutOfRangeException.ThrowIfNegativeOrZero(newRadius);
        ArgumentOutOfRangeException.ThrowIfNegativeOrZero(newAuthorityId);

        return new WatchZone(
            this.Id,
            this.UserId,
            newName,
            newCentre,
            newRadius,
            newAuthorityId,
            this.CreatedAt,
            newPushEnabled,
            newEmailInstantEnabled);
    }
}
