using System.Globalization;
using System.Net.Http.Json;
using System.Reflection;
using System.Text.Json;
using TownCrier.Application.Authorities;

namespace TownCrier.Infrastructure.Geocoding;

public sealed class PostcodesIoAuthorityResolver : IAuthorityResolver
{
    private static readonly Dictionary<string, int> AuthorityMapping = LoadAuthorityMapping();
    private readonly HttpClient httpClient;

    public PostcodesIoAuthorityResolver(HttpClient httpClient)
    {
        this.httpClient = httpClient;
    }

    public async Task<int> ResolveFromCoordinatesAsync(double latitude, double longitude, CancellationToken ct)
    {
        var lat = latitude.ToString(CultureInfo.InvariantCulture);
        var lon = longitude.ToString(CultureInfo.InvariantCulture);
        var requestUri = new Uri($"postcodes?lon={lon}&lat={lat}", UriKind.Relative);

        var response = await this.httpClient.GetAsync(requestUri, ct).ConfigureAwait(false);

        if (!response.IsSuccessStatusCode)
        {
            throw new InvalidOperationException(
                $"Failed to reverse geocode coordinates ({latitude}, {longitude}): {response.StatusCode}");
        }

        var body = await response.Content.ReadFromJsonAsync(
            GeocodingJsonSerializerContext.Default.PostcodesIoReverseResponse,
            ct).ConfigureAwait(false);

        var adminDistrict = body?.Result?.FirstOrDefault()?.AdminDistrict;

        if (string.IsNullOrEmpty(adminDistrict))
        {
            throw new InvalidOperationException(
                $"No local authority found for coordinates ({latitude}, {longitude})");
        }

        if (!AuthorityMapping.TryGetValue(adminDistrict, out var authorityId))
        {
            throw new InvalidOperationException(
                $"No PlanIt authority mapping for admin district '{adminDistrict}' at coordinates ({latitude}, {longitude})");
        }

        return authorityId;
    }

    private static Dictionary<string, int> LoadAuthorityMapping()
    {
        var assembly = Assembly.GetExecutingAssembly();
        const string resourceName = "TownCrier.Infrastructure.Geocoding.authority-mapping.json";

        using var stream = assembly.GetManifestResourceStream(resourceName)
            ?? throw new InvalidOperationException($"Embedded resource '{resourceName}' not found.");

        return JsonSerializer.Deserialize(stream, GeocodingJsonSerializerContext.Default.DictionaryStringInt32)
            ?? throw new InvalidOperationException("Failed to deserialize authority mapping.");
    }
}
