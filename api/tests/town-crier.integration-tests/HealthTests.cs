using System.Net;

namespace TownCrier.IntegrationTests;

[NotInParallel]
public sealed class HealthTests
{
    [Test]
    public async Task Should_Return200WithHealthy_When_GetHealth()
    {
        // Arrange -- unauthenticated client (health endpoint allows anonymous)
        using var client = new HttpClient
        {
            BaseAddress = new Uri(IntegrationTestConfig.ApiBaseUrl),
        };

        // Act
        using var response = await client
            .GetAsync(new Uri("/v1/health", UriKind.Relative))
            .ConfigureAwait(false);
        var body = await response.Content.ReadAsStringAsync().ConfigureAwait(false);

        // Assert
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);
        await Assert.That(body).Contains("Healthy");
    }
}
