using System.Net;
using System.Text.Json;
using Microsoft.AspNetCore.Mvc.Testing;

namespace TownCrier.Web.Tests.Observability;

public sealed class ErrorResponseCorrelationIdTests
{
    [Test]
    public async Task Should_IncludeCorrelationIdInErrorResponse_When_RequestFails()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient(new WebApplicationFactoryClientOptions
        {
            AllowAutoRedirect = false,
        });

        var correlationId = "error-corr-id-789";
        using var request = new HttpRequestMessage(HttpMethod.Get, new Uri("/api/me", UriKind.Relative));
        request.Headers.Add("X-Correlation-Id", correlationId);

        // Act — /api/me requires auth, no token = 401
        using var response = await client.SendAsync(request);

        // Assert
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.Unauthorized);

        var body = await response.Content.ReadAsStringAsync();
        await Assert.That(body).Contains(correlationId);
    }

    [Test]
    public async Task Should_IncludeCorrelationIdInNotFoundResponse_When_RouteNotFound()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        var correlationId = "notfound-corr-id-000";
        using var request = new HttpRequestMessage(HttpMethod.Get, new Uri("/nonexistent", UriKind.Relative));
        request.Headers.Add("X-Correlation-Id", correlationId);

        // Act
        using var response = await client.SendAsync(request);

        // Assert — should get an error status and correlation ID in body
        await Assert.That((int)response.StatusCode).IsGreaterThanOrEqualTo(400);
        var body = await response.Content.ReadAsStringAsync();
        await Assert.That(body).Contains(correlationId);
    }
}
