using System.Net;
using TownCrier.Web.Tests.Auth;

namespace TownCrier.Web.Tests.DependencyInjection;

public sealed class EndpointMappingTests
{
    [Test]
    [Arguments("/health")]
    [Arguments("/v1/health")]
    [Arguments("/v1/version-config")]
    [Arguments("/v1/demo-account")]
    [Arguments("/v1/authorities")]
    public async Task Should_MapAnonymousEndpoints_When_MapAllEndpointsCalled(string path)
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        // Act
        using var response = await client.GetAsync(new Uri(path, UriKind.Relative));

        // Assert — anonymous endpoints must return something other than 404 (route exists)
        await Assert.That(response.StatusCode).IsNotEqualTo(HttpStatusCode.NotFound);
    }

    [Test]
    [Arguments("/v1/me", "POST")]
    [Arguments("/api/me", "GET")]
    public async Task Should_MapAuthenticatedEndpoints_When_MapAllEndpointsCalled(string path, string method)
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();
        var token = TestJwtToken.Generate();
        client.DefaultRequestHeaders.Authorization =
            new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", token);

        // Act
        using var request = new HttpRequestMessage(new HttpMethod(method), new Uri(path, UriKind.Relative));
        using var response = await client.SendAsync(request);

        // Assert — authenticated endpoints must return something other than 404
        await Assert.That(response.StatusCode).IsNotEqualTo(HttpStatusCode.NotFound);
    }

    [Test]
    public async Task Should_ReturnUnauthorized_When_AuthenticatedEndpointCalledWithoutToken()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        // Act
        using var response = await client.GetAsync(new Uri("/v1/me", UriKind.Relative));

        // Assert
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.Unauthorized);
    }
}
