using System.Collections.Concurrent;
using System.Net.Http.Json;
using TownCrier.Application.Geocoding;
using TownCrier.Domain.Geocoding;

namespace TownCrier.Infrastructure.Geocoding;

public sealed class PostcodesIoGeocoder : IPostcodeGeocoder
{
    private readonly HttpClient httpClient;
    private readonly ConcurrentDictionary<string, Coordinates> cache = new();

    public PostcodesIoGeocoder(HttpClient httpClient)
    {
        this.httpClient = httpClient;
    }

    public async Task<Coordinates?> GeocodeAsync(Postcode postcode, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(postcode);

        if (this.cache.TryGetValue(postcode.Value, out var cached))
        {
            return cached;
        }

        var encoded = Uri.EscapeDataString(postcode.Value);
        var requestUri = new Uri($"postcodes/{encoded}", UriKind.Relative);
        var response = await this.httpClient.GetAsync(
            requestUri,
            ct).ConfigureAwait(false);

        if (!response.IsSuccessStatusCode)
        {
            return null;
        }

        var body = await response.Content.ReadFromJsonAsync(
            GeocodingJsonSerializerContext.Default.PostcodesIoResponse,
            ct).ConfigureAwait(false);

        if (body?.Status != 200 || body.Result is null)
        {
            return null;
        }

        var coordinates = new Coordinates(body.Result.Latitude, body.Result.Longitude);
        this.cache.TryAdd(postcode.Value, coordinates);
        return coordinates;
    }
}
