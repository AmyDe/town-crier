using System.Net;
using System.Net.Http.Headers;
using System.Net.Http.Json;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Mvc.Testing;
using Microsoft.AspNetCore.TestHost;
using Microsoft.Extensions.DependencyInjection;
using TownCrier.Application.Auth;
using TownCrier.Application.OfferCodes;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.OfferCodes;
using TownCrier.Domain.UserProfiles;
using TownCrier.Infrastructure.Auth;
using TownCrier.Infrastructure.OfferCodes;
using TownCrier.Web.Endpoints;
using TownCrier.Web.Tests.Auth;

namespace TownCrier.Web.Tests.OfferCodes;

public sealed class RedeemOfferCodeEndpointTests
{
    private const string UserId = "auth0|redeem-user-1";
    private const string CanonicalCode = "ABCDEFGHJKMN";
    private const string DisplayCode = "ABCD-EFGH-JKMN";

    [Test]
    public async Task Should_Return200_WithTierAndExpiry_When_Valid()
    {
        var fixedNow = new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero);
        var repo = new InMemoryOfferCodeRepository();
        await repo.CreateAsync(
            new OfferCode(CanonicalCode, SubscriptionTier.Pro, 30, fixedNow.AddDays(-1)),
            CancellationToken.None).ConfigureAwait(false);

        await using var baseFactory = new TestWebApplicationFactory();
        await using var factory = baseFactory.WithWebHostBuilder(builder =>
            ConfigureOfferCodeHost(builder, repo, fixedNow));

        await SeedFreeUserAsync(factory, UserId).ConfigureAwait(false);
        using var client = CreateAuthenticatedClient(factory);

        var response = await client.PostAsJsonAsync(
            new Uri("/v1/offer-codes/redeem", UriKind.Relative),
            new RedeemOfferCodeRequest(DisplayCode),
            AppJsonSerializerContext.Default.RedeemOfferCodeRequest).ConfigureAwait(false);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);
        var body = await response.Content.ReadFromJsonAsync(
            AppJsonSerializerContext.Default.RedeemOfferCodeResponse).ConfigureAwait(false);
        await Assert.That(body).IsNotNull();
        await Assert.That(body!.Tier).IsEqualTo("Pro");
        await Assert.That(body.ExpiresAt).IsEqualTo(fixedNow.AddDays(30));
    }

    [Test]
    public async Task Should_Return401_When_NoJwtProvided()
    {
        await using var baseFactory = new TestWebApplicationFactory();
        await using var factory = baseFactory.WithWebHostBuilder(builder =>
            ConfigureOfferCodeHost(builder, new InMemoryOfferCodeRepository(), DateTimeOffset.UtcNow));
        using var client = factory.CreateClient();

        var response = await client.PostAsJsonAsync(
            new Uri("/v1/offer-codes/redeem", UriKind.Relative),
            new RedeemOfferCodeRequest(DisplayCode),
            AppJsonSerializerContext.Default.RedeemOfferCodeRequest).ConfigureAwait(false);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.Unauthorized);
    }

    private static Task SeedFreeUserAsync(WebApplicationFactory<Program> factory, string userId)
    {
        var repo = factory.Services.GetRequiredService<IUserProfileRepository>();
        return repo.SaveAsync(UserProfile.Register(userId, $"{userId}@example.com"), CancellationToken.None);
    }

    private static void ConfigureOfferCodeHost(
        IWebHostBuilder builder,
        IOfferCodeRepository repo,
        DateTimeOffset fixedNow)
    {
        builder.ConfigureTestServices(services =>
        {
            services.AddSingleton(repo);
            services.AddSingleton<IOfferCodeGenerator, OfferCodeGenerator>();
            services.AddSingleton<IAuth0ManagementClient, NoOpAuth0ManagementClient>();
            services.AddSingleton<TimeProvider>(new FixedTimeProvider(fixedNow));
        });
    }

    private static HttpClient CreateAuthenticatedClient(
        WebApplicationFactory<Program> factory,
        string userId = UserId)
    {
        var client = factory.CreateClient();
        var token = TestJwtToken.Generate(userId);
        client.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", token);
        return client;
    }

    private sealed class FixedTimeProvider : TimeProvider
    {
        private readonly DateTimeOffset now;

        public FixedTimeProvider(DateTimeOffset now)
        {
            this.now = now;
        }

        public override DateTimeOffset GetUtcNow() => this.now;
    }
}
