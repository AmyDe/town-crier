using System.Net;
using System.Net.Http.Headers;
using Microsoft.AspNetCore.Mvc.Testing;
using Microsoft.AspNetCore.TestHost;
using Microsoft.Extensions.DependencyInjection;
using TownCrier.Application.RateLimiting;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;
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
    [Arguments(SubscriptionTier.Personal)]
    [Arguments(SubscriptionTier.Pro)]
    public async Task Should_ApplyPaidLimit_When_CosmosTierIsPaid(SubscriptionTier tier)
    {
        // Arrange — the user's authoritative tier lives in the Cosmos UserProfile,
        // not the JWT claim (ADR 0010: Cosmos is the single source of truth).
        const string userId = "auth0|paid-user";
        await using var factory = CreateFactoryWithRateLimiting();
        await SeedProfileAsync(factory, userId, tier);
        using var client = CreateClientForUser(factory, userId);

        // Make more requests than the free tier limit
        HttpResponseMessage? lastResponse = null;
        for (var i = 0; i < TestFreeLimit + 1; i++)
        {
            lastResponse?.Dispose();
            lastResponse = await client.GetAsync(new Uri("/api/me", UriKind.Relative));
        }

        // Assert — a paid Cosmos tier gets the higher limit, so this is still allowed
        await Assert.That(lastResponse!.StatusCode).IsEqualTo(HttpStatusCode.OK);
        lastResponse.Dispose();
    }

    [Test]
    public async Task Should_ApplyFreeLimit_When_CosmosTierIsFree()
    {
        // Arrange
        const string userId = "auth0|free-user";
        await using var factory = CreateFactoryWithRateLimiting();
        await SeedProfileAsync(factory, userId, SubscriptionTier.Free);
        using var client = CreateClientForUser(factory, userId);

        // Exhaust the free tier limit
        for (var i = 0; i < TestFreeLimit; i++)
        {
            var warmup = await client.GetAsync(new Uri("/api/me", UriKind.Relative));
            warmup.Dispose();
        }

        // Act — this request should be over the free limit
        using var response = await client.GetAsync(new Uri("/api/me", UriKind.Relative));

        // Assert — a Free Cosmos tier only gets the free limit
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.TooManyRequests);
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
        return CreateClientForUser(factory, "auth0|test-user-123");
    }

    private static HttpClient CreateClientForUser(WebApplicationFactory<Program> factory, string userId)
    {
        var client = factory.CreateClient();
        var token = TestJwtToken.Generate(userId);
        client.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", token);
        return client;
    }

    private static async Task SeedProfileAsync(
        WebApplicationFactory<Program> factory, string userId, SubscriptionTier tier)
    {
        var repository = factory.Services.GetRequiredService<IUserProfileRepository>();
        var profile = UserProfile.Register(userId);
        if (tier != SubscriptionTier.Free)
        {
            profile.ActivateSubscription(tier, DateTimeOffset.UtcNow.AddDays(30));
        }

        await repository.SaveAsync(profile, CancellationToken.None);
    }
}
