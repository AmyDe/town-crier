using System.Net;

namespace TownCrier.Web.Tests.Cors;

public sealed class CorsMiddlewareTests
{
    [Test]
    public async Task Should_ReturnCorsHeaders_When_RequestFromAllowedOrigin()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();
        using var request = new HttpRequestMessage(HttpMethod.Get, new Uri("/health", UriKind.Relative));
        request.Headers.Add("Origin", "http://localhost:5173");

        // Act
        using var response = await client.SendAsync(request);

        // Assert
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);
        await Assert.That(response.Headers.Contains("Access-Control-Allow-Origin")).IsTrue();
        var allowOrigin = response.Headers.GetValues("Access-Control-Allow-Origin").First();
        await Assert.That(allowOrigin).IsEqualTo("http://localhost:5173");
    }

    [Test]
    public async Task Should_NotReturnCorsHeaders_When_RequestFromDisallowedOrigin()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();
        using var request = new HttpRequestMessage(HttpMethod.Get, new Uri("/health", UriKind.Relative));
        request.Headers.Add("Origin", "https://evil.example.com");

        // Act
        using var response = await client.SendAsync(request);

        // Assert
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);
        await Assert.That(response.Headers.Contains("Access-Control-Allow-Origin")).IsFalse();
    }

    [Test]
    public async Task Should_HandlePreflightRequest_When_OptionsFromAllowedOrigin()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();
        using var request = new HttpRequestMessage(HttpMethod.Options, new Uri("/v1/health", UriKind.Relative));
        request.Headers.Add("Origin", "http://localhost:5173");
        request.Headers.Add("Access-Control-Request-Method", "GET");

        // Act
        using var response = await client.SendAsync(request);

        // Assert
        await Assert.That(response.Headers.Contains("Access-Control-Allow-Origin")).IsTrue();
        var allowOrigin = response.Headers.GetValues("Access-Control-Allow-Origin").First();
        await Assert.That(allowOrigin).IsEqualTo("http://localhost:5173");
    }
}
