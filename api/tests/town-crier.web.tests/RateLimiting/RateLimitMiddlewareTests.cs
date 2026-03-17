using System.Net;
using System.Net.Http.Headers;
using Microsoft.AspNetCore.Mvc.Testing;
using Microsoft.AspNetCore.TestHost;
using Microsoft.Extensions.DependencyInjection;
using TownCrier.Application.RateLimiting;
using TownCrier.Web.Tests.Auth;

namespace TownCrier.Web.Tests.RateLimiting;

public sealed class RateLimitMiddlewareTests
{
    private const int TestFreeLimit = 3;
    private const int TestPaidLimit = 10;

    [Test]
    public async Task Should_AllowRequest_When_UnderRateLimit()
    {
        // Arrange
        await using var factory = CreateFactoryWithRateLimiting();
        using var client = CreateAuthenticatedClient(factory);

        // Act
        using var response = await client.GetAsync(new Uri("/api/me", UriKind.Relative));

        // Assert
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);
    }

    [Test]
    public async Task Should_Return429WithRetryAfter_When_RateLimitExceeded()
    {
        // Arrange
        await using var factory = CreateFactoryWithRateLimiting();
        using var client = CreateAuthenticatedClient(factory);

        // Exhaust the free tier limit
        for (var i = 0; i < TestFreeLimit; i++)
        {
            var warmup = await client.GetAsync(new Uri("/api/me", UriKind.Relative));
            warmup.Dispose();
        }

        // Act — this request should be over the limit
        using var response = await client.GetAsync(new Uri("/api/me", UriKind.Relative));

        // Assert
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.TooManyRequests);
        await Assert.That(response.Headers.RetryAfter).IsNotNull();
    }

    [Test]
    public async Task Should_NotRateLimit_When_EndpointIsAnonymous()
    {
        // Arrange
        await using var factory = CreateFactoryWithRateLimiting();
        using var client = factory.CreateClient();

        // Act — make more requests than the free limit
        HttpResponseMessage? lastResponse = null;
        for (var i = 0; i <= TestFreeLimit + 1; i++)
        {
            lastResponse?.Dispose();
            lastResponse = await client.GetAsync(new Uri("/health", UriKind.Relative));
        }

        // Assert — anonymous endpoints should never be rate-limited
        await Assert.That(lastResponse!.StatusCode).IsEqualTo(HttpStatusCode.OK);
        lastResponse.Dispose();
    }

    [Test]
    public async Task Should_AllowMoreRequests_When_PaidTier()
    {
        // Arrange
        await using var factory = CreateFactoryWithRateLimiting();
        var token = TestJwtToken.Generate(claims: [new("subscription_tier", "paid")]);
        using var client = factory.CreateClient();
        client.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", token);

        // Make more requests than the free tier limit
        HttpResponseMessage? lastResponse = null;
        for (var i = 0; i < TestFreeLimit + 1; i++)
        {
            lastResponse?.Dispose();
            lastResponse = await client.GetAsync(new Uri("/api/me", UriKind.Relative));
        }

        // Assert — paid tier has a higher limit, so this should still be allowed
        await Assert.That(lastResponse!.StatusCode).IsEqualTo(HttpStatusCode.OK);
        lastResponse.Dispose();
    }

    [Test]
    public async Task Should_IncludeRateLimitHeaders_When_RequestIsAuthenticated()
    {
        // Arrange
        await using var factory = CreateFactoryWithRateLimiting();
        using var client = CreateAuthenticatedClient(factory);

        // Act
        using var response = await client.GetAsync(new Uri("/api/me", UriKind.Relative));

        // Assert
        await Assert.That(response.Headers.Contains("X-RateLimit-Limit")).IsTrue();
        await Assert.That(response.Headers.Contains("X-RateLimit-Remaining")).IsTrue();
    }

#pragma warning disable CA2000 // TestWebApplicationFactory is disposed via the returned WebApplicationFactory
    private static WebApplicationFactory<Program> CreateFactoryWithRateLimiting()
    {
        return new TestWebApplicationFactory().WithWebHostBuilder(builder =>
#pragma warning restore CA2000
            builder.ConfigureTestServices(services =>
                services.Configure<RateLimitOptions>(options =>
                {
                    options.Window = TimeSpan.FromMinutes(1);
                    options.FreeTierLimit = TestFreeLimit;
                    options.PaidTierLimit = TestPaidLimit;
                })));
    }

    private static HttpClient CreateAuthenticatedClient(WebApplicationFactory<Program> factory)
    {
        var client = factory.CreateClient();
        var token = TestJwtToken.Generate();
        client.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", token);
        return client;
    }
}
