using System.Net;

namespace TownCrier.IntegrationTests;

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

        // Warm up — staging revision may need a cold-start window
        const int maxAttempts = 5;
        HttpResponseMessage response = null!;
        for (var i = 0; i < maxAttempts; i++)
        {
            response = await client
                .GetAsync(new Uri("/v1/health", UriKind.Relative))
                .ConfigureAwait(false);

            if (response.StatusCode == HttpStatusCode.OK || i == maxAttempts - 1)
            {
                break;
            }

            response.Dispose();
            await Task.Delay(TimeSpan.FromSeconds(2 * (i + 1))).ConfigureAwait(false);
        }

        // Act
        var body = await response.Content.ReadAsStringAsync().ConfigureAwait(false);

        // Assert
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);
        await Assert.That(body).Contains("Healthy");
    }
}
