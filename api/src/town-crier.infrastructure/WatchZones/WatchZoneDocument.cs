using TownCrier.Domain.Geocoding;
using TownCrier.Domain.WatchZones;

namespace TownCrier.Infrastructure.WatchZones;

internal sealed class WatchZoneDocument
{
    public required string Id { get; init; }

    public required string UserId { get; init; }

    public required string Name { get; init; }

    public required double Latitude { get; init; }

    public required double Longitude { get; init; }

    public required double RadiusMetres { get; init; }

    public required int AuthorityId { get; init; }

    public DateTimeOffset? CreatedAt { get; init; }

    // Nullable so existing Cosmos documents predating per-zone notification flags
    // (tc-kh1s) hydrate as opt-in to push and email — preserving prior behaviour.
    // The System.Text.Json source generator sets `bool` properties to `default(bool)`
    // (false) when the JSON field is missing — even when a property initializer
    // declares `= true` — so we use `bool?` and coalesce at `ToDomain` time.
    public bool? PushEnabled { get; init; }

    public bool? EmailInstantEnabled { get; init; }

    public static WatchZoneDocument FromDomain(WatchZone zone)
    {
        ArgumentNullException.ThrowIfNull(zone);

        return new WatchZoneDocument
        {
            Id = zone.Id,
            UserId = zone.UserId,
            Name = zone.Name,
            Latitude = zone.Centre.Latitude,
            Longitude = zone.Centre.Longitude,
            RadiusMetres = zone.RadiusMetres,
            AuthorityId = zone.AuthorityId,
            CreatedAt = zone.CreatedAt,
            PushEnabled = zone.PushEnabled,
            EmailInstantEnabled = zone.EmailInstantEnabled,
        };
    }

    public WatchZone ToDomain()
    {
        return new WatchZone(
            this.Id,
            this.UserId,
            this.Name,
            new Coordinates(this.Latitude, this.Longitude),
            this.RadiusMetres,
            this.AuthorityId,
            this.CreatedAt ?? DateTimeOffset.MinValue,
            this.PushEnabled ?? true,
            this.EmailInstantEnabled ?? true);
    }
}
