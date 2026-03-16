using System.Net;
using Microsoft.AspNetCore.Mvc.Testing;

namespace TownCrier.Web.Tests.Health;

public sealed class HealthEndpointTests
{
    [Test]
    public async Task Should_ReturnOk_When_HealthCalledAtRootPath()
    {
        // Arrange
        await using var factory = new WebApplicationFactory<Program>();
        using var client = factory.CreateClient();

        // Act
        using var response = await client.GetAsync(new Uri("/health", UriKind.Relative));

        // Assert
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);
    }

    [Test]
    public async Task Should_ReturnOk_When_HealthCalledAtVersionedPath()
    {
        // Arrange
        await using var factory = new WebApplicationFactory<Program>();
        using var client = factory.CreateClient();

        // Act
        using var response = await client.GetAsync(new Uri("/v1/health", UriKind.Relative));

        // Assert
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);
    }

    [Test]
    public async Task Should_ReturnHealthyBody_When_HealthCalledAtVersionedPath()
    {
        // Arrange
        await using var factory = new WebApplicationFactory<Program>();
        using var client = factory.CreateClient();

        // Act
        using var response = await client.GetAsync(new Uri("/v1/health", UriKind.Relative));
        var body = await response.Content.ReadAsStringAsync();

        // Assert
        await Assert.That(body).Contains("Healthy");
    }
}
