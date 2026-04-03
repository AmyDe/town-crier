using System.Net;
using Microsoft.AspNetCore.Mvc.Testing;

namespace TownCrier.Web.Tests.Observability;

public sealed class ErrorResponseMiddlewareTests
{
    [Test]
    public async Task Should_ReturnStructuredError_When_UnauthorizedRequest()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient(new WebApplicationFactoryClientOptions
        {
            AllowAutoRedirect = false,
        });

        // Act — /v1/me requires auth, no token = 401
        using var response = await client.GetAsync(new Uri("/v1/me", UriKind.Relative));

        // Assert
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.Unauthorized);
        var body = await response.Content.ReadAsStringAsync();
        await Assert.That(body).Contains("Unauthorized");
    }

    [Test]
    public async Task Should_ReturnStructuredError_When_RouteNotFound()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        // Act
        using var response = await client.GetAsync(new Uri("/nonexistent", UriKind.Relative));

        // Assert
        await Assert.That((int)response.StatusCode).IsGreaterThanOrEqualTo(400);
        var body = await response.Content.ReadAsStringAsync();
        await Assert.That(body).Contains("Status");
    }
}
