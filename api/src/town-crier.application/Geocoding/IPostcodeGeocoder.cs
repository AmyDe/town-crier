using TownCrier.Domain.Geocoding;

namespace TownCrier.Application.Geocoding;

public interface IPostcodeGeocoder
{
    Task<Coordinates?> GeocodeAsync(Postcode postcode, CancellationToken ct);
}
