using TownCrier.Application.Geocoding;
using TownCrier.Domain.Geocoding;

namespace TownCrier.Application.Tests.Geocoding;

internal sealed class FakePostcodeGeocoder : IPostcodeGeocoder
{
    private readonly Dictionary<string, Coordinates> store = new();

    public void Add(Postcode postcode, Coordinates coordinates)
    {
        this.store[postcode.Value] = coordinates;
    }

    public Task<Coordinates?> GeocodeAsync(Postcode postcode, CancellationToken ct)
    {
        this.store.TryGetValue(postcode.Value, out var coordinates);
        return Task.FromResult(coordinates);
    }
}
