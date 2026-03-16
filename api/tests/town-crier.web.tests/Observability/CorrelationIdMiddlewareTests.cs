using System.Net;

namespace TownCrier.Web.Tests.Observability;

public sealed class CorrelationIdMiddlewareTests
{
    [Test]
    public async Task Should_ReturnCorrelationIdHeader_When_RequestMade()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        // Act
        using var response = await client.GetAsync(new Uri("/health", UriKind.Relative));

        // Assert
        await Assert.That(response.Headers.Contains("X-Correlation-Id")).IsTrue();
        var correlationId = response.Headers.GetValues("X-Correlation-Id").First();
        await Assert.That(correlationId).IsNotEmpty();
    }

    [Test]
    public async Task Should_EchoCorrelationId_When_ClientSendsOne()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();
        var expectedId = "test-correlation-123";

        using var request = new HttpRequestMessage(HttpMethod.Get, new Uri("/health", UriKind.Relative));
        request.Headers.Add("X-Correlation-Id", expectedId);

        // Act
        using var response = await client.SendAsync(request);

        // Assert
        var returnedId = response.Headers.GetValues("X-Correlation-Id").First();
        await Assert.That(returnedId).IsEqualTo(expectedId);
    }
}
