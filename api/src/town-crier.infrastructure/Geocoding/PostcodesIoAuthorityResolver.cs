using System.Globalization;
using System.Net.Http.Json;
using TownCrier.Application.Authorities;

namespace TownCrier.Infrastructure.Geocoding;

public sealed class PostcodesIoAuthorityResolver : IAuthorityResolver
{
    private readonly IAuthorityProvider authorityProvider;
    private readonly HttpClient httpClient;

    public PostcodesIoAuthorityResolver(HttpClient httpClient, IAuthorityProvider authorityProvider)
    {
        this.httpClient = httpClient;
        this.authorityProvider = authorityProvider;
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

        var authorities = await this.authorityProvider.GetAllAsync(ct).ConfigureAwait(false);
        var match = authorities.FirstOrDefault(a =>
            string.Equals(a.Name, adminDistrict, StringComparison.OrdinalIgnoreCase));

        if (match is null)
        {
            throw new InvalidOperationException(
                $"No PlanIt authority matches local authority '{adminDistrict}' for coordinates ({latitude}, {longitude})");
        }

        return match.Id;
    }
}
