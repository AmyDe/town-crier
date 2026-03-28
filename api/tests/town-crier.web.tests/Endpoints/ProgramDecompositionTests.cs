using System.Net;
using TownCrier.Web.Tests.Auth;

namespace TownCrier.Web.Tests.Endpoints;

/// <summary>
/// These tests verify that after Program.cs is decomposed into extension methods,
/// the application still starts correctly and all endpoint groups respond as expected.
/// Each test validates a distinct endpoint group that was extracted from Program.cs.
/// </summary>
public sealed class ProgramDecompositionTests
{
    [Test]
    public async Task Should_ReturnOk_When_HealthEndpointsMapped()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        // Act
        using var rootHealth = await client.GetAsync(new Uri("/health", UriKind.Relative));
        using var v1Health = await client.GetAsync(new Uri("/v1/health", UriKind.Relative));

        // Assert
        await Assert.That(rootHealth.StatusCode).IsEqualTo(HttpStatusCode.OK);
        await Assert.That(v1Health.StatusCode).IsEqualTo(HttpStatusCode.OK);
    }

    [Test]
    public async Task Should_ReturnOk_When_VersionConfigEndpointMapped()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        // Act
        using var response = await client.GetAsync(new Uri("/v1/version-config", UriKind.Relative));

        // Assert
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);
    }

    [Test]
    public async Task Should_ReturnOk_When_DemoAccountEndpointMapped()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        // Act
        using var response = await client.GetAsync(new Uri("/v1/demo-account", UriKind.Relative));

        // Assert
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);
    }

    [Test]
    public async Task Should_MapMeEndpoints_When_ApplicationStarts()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();
        var token = TestJwtToken.Generate("test-user-decomp");
        client.DefaultRequestHeaders.Authorization =
            new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", token);

        // Act - GET /v1/me verifies the route is mapped (profile may not exist yet)
        using var getResponse = await client.GetAsync(
            new Uri("/v1/me", UriKind.Relative));

        // Assert - should not be 404 (route exists) or 401 (auth works)
        await Assert.That(getResponse.StatusCode).IsNotEqualTo(HttpStatusCode.NotFound);
        await Assert.That(getResponse.StatusCode).IsNotEqualTo(HttpStatusCode.Unauthorized);
    }

    [Test]
    public async Task Should_MapWatchZoneEndpoints_When_ApplicationStarts()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();
        var token = TestJwtToken.Generate("test-user-zones");
        client.DefaultRequestHeaders.Authorization =
            new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", token);

        // Act
        using var response = await client.GetAsync(
            new Uri("/v1/me/watch-zones", UriKind.Relative));

        // Assert
        await Assert.That(response.StatusCode).IsNotEqualTo(HttpStatusCode.NotFound);
        await Assert.That(response.StatusCode).IsNotEqualTo(HttpStatusCode.Unauthorized);
    }

    [Test]
    public async Task Should_MapSavedApplicationEndpoints_When_ApplicationStarts()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();
        var token = TestJwtToken.Generate("test-user-saved");
        client.DefaultRequestHeaders.Authorization =
            new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", token);

        // Act
        using var response = await client.GetAsync(
            new Uri("/v1/me/saved-applications", UriKind.Relative));

        // Assert
        await Assert.That(response.StatusCode).IsNotEqualTo(HttpStatusCode.NotFound);
        await Assert.That(response.StatusCode).IsNotEqualTo(HttpStatusCode.Unauthorized);
    }

    [Test]
    public async Task Should_MapGroupEndpoints_When_ApplicationStarts()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();
        var token = TestJwtToken.Generate("test-user-groups");
        client.DefaultRequestHeaders.Authorization =
            new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", token);

        // Act
        using var response = await client.GetAsync(
            new Uri("/v1/groups", UriKind.Relative));

        // Assert
        await Assert.That(response.StatusCode).IsNotEqualTo(HttpStatusCode.NotFound);
        await Assert.That(response.StatusCode).IsNotEqualTo(HttpStatusCode.Unauthorized);
    }

    [Test]
    public async Task Should_MapApiMeEndpoint_When_ApplicationStarts()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();
        var token = TestJwtToken.Generate("test-user-api");
        client.DefaultRequestHeaders.Authorization =
            new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", token);

        // Act
        using var response = await client.GetAsync(new Uri("/api/me", UriKind.Relative));

        // Assert - route exists and accepts authenticated requests
        await Assert.That(response.StatusCode).IsNotEqualTo(HttpStatusCode.NotFound);
        await Assert.That(response.StatusCode).IsNotEqualTo(HttpStatusCode.Unauthorized);
    }
}
