using TownCrier.Domain.Geocoding;

namespace TownCrier.Application.Geocoding;

public sealed class GeocodePostcodeQueryHandler
{
    private readonly IPostcodeGeocoder geocoder;

    public GeocodePostcodeQueryHandler(IPostcodeGeocoder geocoder)
    {
        this.geocoder = geocoder;
    }

    public async Task<GeocodePostcodeResult> HandleAsync(GeocodePostcodeQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        var postcode = Postcode.Create(query.Postcode);

        var coordinates = await this.geocoder.GeocodeAsync(postcode, ct).ConfigureAwait(false)
            ?? throw new InvalidOperationException($"Postcode '{query.Postcode}' could not be geocoded.");

        return new GeocodePostcodeResult(coordinates);
    }
}
