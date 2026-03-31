using System.Net;
using System.Net.Http.Json;
using System.Text.Json;

namespace TownCrier.IntegrationTests;

[NotInParallel]
public sealed class WatchZoneTests
{
    private static readonly JsonSerializerOptions CamelCaseOptions = new()
    {
        PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
    };

    [Test]
    [ClassDataSource<ApiClientFixture>(Shared = SharedType.PerTestSession)]
    public async Task Should_CreateAndListWatchZone_When_Authenticated(ApiClientFixture fixture)
    {
        ArgumentNullException.ThrowIfNull(fixture);

        var client = fixture.Client;

        // Defensively create profile first (watch zones require a profile)
        await client.PostAsync(new Uri("/v1/me", UriKind.Relative), null).ConfigureAwait(false);

        // Arrange -- unique zone ID
        var zoneId = Guid.NewGuid().ToString();
        var createPayload = new
        {
            zoneId,
            name = "Integration Test Zone",
            latitude = 51.5074,
            longitude = -0.1278,
            radiusMetres = 1000.0,
        };

        // Act -- create watch zone
        using var createResponse = await client
            .PostAsJsonAsync(new Uri("/v1/me/watch-zones", UriKind.Relative), createPayload, CamelCaseOptions)
            .ConfigureAwait(false);

        // Assert -- create returns 201
        await Assert.That(createResponse.StatusCode).IsEqualTo(HttpStatusCode.Created);

        // Act -- list watch zones
        using var listResponse = await client
            .GetAsync(new Uri("/v1/me/watch-zones", UriKind.Relative))
            .ConfigureAwait(false);
        var listBody = await listResponse.Content.ReadAsStringAsync().ConfigureAwait(false);

        // Assert -- list returns 200 with the zone
        await Assert.That(listResponse.StatusCode).IsEqualTo(HttpStatusCode.OK);

        // Verify camelCase serialization and the zone is present
        using var doc = JsonDocument.Parse(listBody);
        var zones = doc.RootElement.GetProperty("zones");
        await Assert.That(zones.GetArrayLength()).IsGreaterThanOrEqualTo(1);

        var found = false;
        foreach (var zone in zones.EnumerateArray())
        {
            if (zone.GetProperty("id").GetString() == zoneId)
            {
                found = true;

                // Verify camelCase property names exist
                await Assert.That(zone.TryGetProperty("name", out _)).IsTrue();
                await Assert.That(zone.TryGetProperty("latitude", out _)).IsTrue();
                await Assert.That(zone.TryGetProperty("longitude", out _)).IsTrue();
                await Assert.That(zone.TryGetProperty("radiusMetres", out _)).IsTrue();
                break;
            }
        }

        await Assert.That(found).IsTrue();

        // Cleanup -- delete the watch zone
        await client
            .DeleteAsync(new Uri($"/v1/me/watch-zones/{zoneId}", UriKind.Relative))
            .ConfigureAwait(false);
    }
}
