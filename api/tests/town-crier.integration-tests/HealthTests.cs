using System.Net;

namespace TownCrier.IntegrationTests;

[NotInParallel]
public sealed class HealthTests
{
    [Test]
    [DisplayName("Diagnostic: verify env vars visible to .NET")]
    public async Task Should_SeeEnvironmentVariables()
    {
        var domain = Environment.GetEnvironmentVariable("INTEGRATION_TEST_AUTH0_DOMAIN");
        var baseUrl = Environment.GetEnvironmentVariable("INTEGRATION_TEST_API_BASE_URL");

        Console.WriteLine($"INTEGRATION_TEST_AUTH0_DOMAIN = '{domain}'");
        Console.WriteLine($"INTEGRATION_TEST_API_BASE_URL = '{baseUrl}'");

        // Dump all env vars with INTEGRATION prefix
        foreach (System.Collections.DictionaryEntry e in Environment.GetEnvironmentVariables())
        {
            if (e.Key is string key && key.StartsWith("INTEGRATION", StringComparison.Ordinal))
            {
                Console.WriteLine($"  {key} = <set>");
            }
        }

        await Assert.That(domain).IsNotNull();
    }

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
