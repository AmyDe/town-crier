using System.Net;
using TownCrier.Web.Tests.Auth;

namespace TownCrier.Web.Tests.Endpoints;

public sealed class EndpointMappingTests
{
    [Test]
    public async Task Should_MapHealthEndpoints_When_ApplicationStarts()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        // Act
        using var rootHealth = await client.GetAsync(new Uri("/health", UriKind.Relative));
        using var v1Health = await client.GetAsync(new Uri("/v1/health", UriKind.Relative));

        // Assert - health endpoints exist and are accessible without auth
        await Assert.That(rootHealth.StatusCode).IsEqualTo(HttpStatusCode.OK);
        await Assert.That(v1Health.StatusCode).IsEqualTo(HttpStatusCode.OK);
    }

    [Test]
    public async Task Should_MapAnonymousEndpoints_When_ApplicationStarts()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        // Act
        using var authorities = await client.GetAsync(new Uri("/v1/authorities", UriKind.Relative));
        using var versionConfig = await client.GetAsync(new Uri("/v1/version-config", UriKind.Relative));
        using var demoAccount = await client.GetAsync(new Uri("/v1/demo-account", UriKind.Relative));

        // Assert - anonymous endpoints should not return 401 or 404
        await Assert.That(authorities.StatusCode).IsNotEqualTo(HttpStatusCode.NotFound);
        await Assert.That(authorities.StatusCode).IsNotEqualTo(HttpStatusCode.Unauthorized);
        await Assert.That(versionConfig.StatusCode).IsNotEqualTo(HttpStatusCode.NotFound);
        await Assert.That(versionConfig.StatusCode).IsNotEqualTo(HttpStatusCode.Unauthorized);
        await Assert.That(demoAccount.StatusCode).IsNotEqualTo(HttpStatusCode.NotFound);
        await Assert.That(demoAccount.StatusCode).IsNotEqualTo(HttpStatusCode.Unauthorized);
    }

    [Test]
    public async Task Should_RequireAuthentication_When_AccessingProtectedEndpoints()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        // Act
        using var me = await client.GetAsync(new Uri("/v1/me", UriKind.Relative));
        using var notifications = await client.GetAsync(new Uri("/v1/notifications", UriKind.Relative));
        using var groups = await client.GetAsync(new Uri("/v1/groups", UriKind.Relative));
        using var savedApps = await client.GetAsync(new Uri("/v1/me/saved-applications", UriKind.Relative));
        using var watchZones = await client.GetAsync(new Uri("/v1/me/watch-zones", UriKind.Relative));
        using var apiMe = await client.GetAsync(new Uri("/api/me", UriKind.Relative));

        // Assert - all protected endpoints require auth
        await Assert.That(me.StatusCode).IsEqualTo(HttpStatusCode.Unauthorized);
        await Assert.That(notifications.StatusCode).IsEqualTo(HttpStatusCode.Unauthorized);
        await Assert.That(groups.StatusCode).IsEqualTo(HttpStatusCode.Unauthorized);
        await Assert.That(savedApps.StatusCode).IsEqualTo(HttpStatusCode.Unauthorized);
        await Assert.That(watchZones.StatusCode).IsEqualTo(HttpStatusCode.Unauthorized);
        await Assert.That(apiMe.StatusCode).IsEqualTo(HttpStatusCode.Unauthorized);
    }

    [Test]
    public async Task Should_MapAuthenticatedEndpoints_When_ValidTokenProvided()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();
        var token = TestJwtToken.Generate("test-user-123");
        client.DefaultRequestHeaders.Authorization =
            new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", token);

        // Act
        using var me = await client.GetAsync(new Uri("/v1/me", UriKind.Relative));
        using var watchZones = await client.GetAsync(new Uri("/v1/me/watch-zones", UriKind.Relative));
        using var savedApps = await client.GetAsync(new Uri("/v1/me/saved-applications", UriKind.Relative));
        using var groups = await client.GetAsync(new Uri("/v1/groups", UriKind.Relative));
        using var apiMe = await client.GetAsync(new Uri("/api/me", UriKind.Relative));

        // Assert - authenticated endpoints should not return 401 or 404
        await Assert.That(me.StatusCode).IsNotEqualTo(HttpStatusCode.Unauthorized);
        await Assert.That(me.StatusCode).IsNotEqualTo(HttpStatusCode.NotFound);
        await Assert.That(watchZones.StatusCode).IsNotEqualTo(HttpStatusCode.Unauthorized);
        await Assert.That(watchZones.StatusCode).IsNotEqualTo(HttpStatusCode.NotFound);
        await Assert.That(savedApps.StatusCode).IsNotEqualTo(HttpStatusCode.Unauthorized);
        await Assert.That(savedApps.StatusCode).IsNotEqualTo(HttpStatusCode.NotFound);
        await Assert.That(groups.StatusCode).IsNotEqualTo(HttpStatusCode.Unauthorized);
        await Assert.That(groups.StatusCode).IsNotEqualTo(HttpStatusCode.NotFound);
        await Assert.That(apiMe.StatusCode).IsNotEqualTo(HttpStatusCode.Unauthorized);
        await Assert.That(apiMe.StatusCode).IsNotEqualTo(HttpStatusCode.NotFound);
    }
}
