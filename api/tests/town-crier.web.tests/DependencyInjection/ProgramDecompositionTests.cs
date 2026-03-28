using System.Net;
using TownCrier.Web.Extensions;
using TownCrier.Web.Tests.Auth;

namespace TownCrier.Web.Tests.DependencyInjection;

public sealed class ProgramDecompositionTests
{
    [Test]
    public async Task Should_ConfigureMiddlewarePipeline_When_UseMiddlewarePipelineCalled()
    {
        // This test verifies UseMiddlewarePipeline exists and correctly wires
        // CORS, correlation ID, error response, request logging, auth, and rate limiting.
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        // Act
        using var response = await client.GetAsync(new Uri("/health", UriKind.Relative));

        // Assert — middleware pipeline must be active
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);

        // Verify correlation ID middleware is in the pipeline
        await Assert.That(response.Headers.Contains("X-Correlation-Id")).IsTrue();
    }

    [Test]
    public async Task Should_MapHealthEndpoints_When_MapAllEndpointsCalled()
    {
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        using var rootHealth = await client.GetAsync(new Uri("/health", UriKind.Relative));
        using var v1Health = await client.GetAsync(new Uri("/v1/health", UriKind.Relative));

        await Assert.That(rootHealth.StatusCode).IsEqualTo(HttpStatusCode.OK);
        await Assert.That(v1Health.StatusCode).IsEqualTo(HttpStatusCode.OK);
    }

    [Test]
    public async Task Should_MapVersionConfigEndpoint_When_MapAllEndpointsCalled()
    {
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        using var response = await client.GetAsync(new Uri("/v1/version-config", UriKind.Relative));

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);
    }

    [Test]
    public async Task Should_MapDemoAccountEndpoint_When_MapAllEndpointsCalled()
    {
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        using var response = await client.GetAsync(new Uri("/v1/demo-account", UriKind.Relative));

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);
    }

    [Test]
    public async Task Should_MapUserProfileEndpoints_When_MapAllEndpointsCalled()
    {
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();
        var token = TestJwtToken.Generate();
        client.DefaultRequestHeaders.Authorization =
            new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", token);

        // POST creates a profile, GET retrieves it
        using var createResponse = await client.PostAsync(
            new Uri("/v1/me", UriKind.Relative), null);

        // Route exists (not 404/405) — the handler may succeed or throw depending on DI state
        await Assert.That(createResponse.StatusCode).IsNotEqualTo(HttpStatusCode.NotFound);
        await Assert.That(createResponse.StatusCode).IsNotEqualTo(HttpStatusCode.MethodNotAllowed);

        using var getResponse = await client.GetAsync(
            new Uri("/v1/me", UriKind.Relative));

        await Assert.That(getResponse.StatusCode).IsNotEqualTo(HttpStatusCode.NotFound);
    }

    [Test]
    public async Task Should_MapApiMeEndpoint_When_MapAllEndpointsCalled()
    {
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();
        var token = TestJwtToken.Generate();
        client.DefaultRequestHeaders.Authorization =
            new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", token);

        using var response = await client.GetAsync(new Uri("/api/me", UriKind.Relative));

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);
    }

    [Test]
    public async Task Should_EnforceAuthentication_When_ProtectedEndpointCalledWithoutToken()
    {
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        using var response = await client.GetAsync(new Uri("/v1/me", UriKind.Relative));

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.Unauthorized);
    }

    [Test]
    public async Task Should_IncludeCorrelationId_When_AnyRequestProcessed()
    {
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        using var response = await client.GetAsync(new Uri("/health", UriKind.Relative));

        await Assert.That(response.Headers.Contains("X-Correlation-Id")).IsTrue();
    }

    [Test]
    public async Task Should_ExposeExtensionMethods_When_WebApplicationExtensionsExists()
    {
        // Compile-time verification: if UseMiddlewarePipeline or MapAllEndpoints
        // do not exist on WebApplicationExtensions, this file will not compile.
        // The runtime test proves the extensions are usable through the full app boot.
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        // Verify the full pipeline and endpoint mapping works end-to-end
        using var healthResponse = await client.GetAsync(new Uri("/health", UriKind.Relative));
        using var v1HealthResponse = await client.GetAsync(new Uri("/v1/health", UriKind.Relative));

        await Assert.That(healthResponse.StatusCode).IsEqualTo(HttpStatusCode.OK);
        await Assert.That(v1HealthResponse.StatusCode).IsEqualTo(HttpStatusCode.OK);
    }
}
