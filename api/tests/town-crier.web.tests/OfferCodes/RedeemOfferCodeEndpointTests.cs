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
    public async Task Should_Return400_WithInvalidCodeFormat_When_CodeMalformed()
    {
        var fixedNow = new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero);
        var repo = new InMemoryOfferCodeRepository();

        await using var baseFactory = new TestWebApplicationFactory();
        await using var factory = baseFactory.WithWebHostBuilder(builder =>
            ConfigureOfferCodeHost(builder, repo, fixedNow));

        await SeedFreeUserAsync(factory, UserId).ConfigureAwait(false);
        using var client = CreateAuthenticatedClient(factory);

        var response = await client.PostAsJsonAsync(
            new Uri("/v1/offer-codes/redeem", UriKind.Relative),
            new RedeemOfferCodeRequest("TOO-SHORT"),
            AppJsonSerializerContext.Default.RedeemOfferCodeRequest).ConfigureAwait(false);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.BadRequest);
        var body = await response.Content.ReadFromJsonAsync(
            AppJsonSerializerContext.Default.ApiErrorResponse).ConfigureAwait(false);
        await Assert.That(body).IsNotNull();
        await Assert.That(body!.Error).IsEqualTo("invalid_code_format");
        await Assert.That(body.Message).IsNotNull();
    }

    [Test]
    public async Task Should_Return404_WithInvalidCode_When_CodeNotFound()
    {
        var fixedNow = new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero);
        var repo = new InMemoryOfferCodeRepository();

        await using var baseFactory = new TestWebApplicationFactory();
        await using var factory = baseFactory.WithWebHostBuilder(builder =>
            ConfigureOfferCodeHost(builder, repo, fixedNow));

        await SeedFreeUserAsync(factory, UserId).ConfigureAwait(false);
        using var client = CreateAuthenticatedClient(factory);

        var response = await client.PostAsJsonAsync(
            new Uri("/v1/offer-codes/redeem", UriKind.Relative),
            new RedeemOfferCodeRequest(DisplayCode),
            AppJsonSerializerContext.Default.RedeemOfferCodeRequest).ConfigureAwait(false);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.NotFound);
        var body = await response.Content.ReadFromJsonAsync(
            AppJsonSerializerContext.Default.ApiErrorResponse).ConfigureAwait(false);
        await Assert.That(body).IsNotNull();
        await Assert.That(body!.Error).IsEqualTo("invalid_code");
        await Assert.That(body.Message).IsNotNull();
    }

    [Test]
    public async Task Should_Return409_WithCodeAlreadyRedeemed_When_CodeAlreadyUsed()
    {
        var fixedNow = new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero);
        var repo = new InMemoryOfferCodeRepository();
        var redeemed = new OfferCode(
            CanonicalCode,
            SubscriptionTier.Pro,
            30,
            fixedNow.AddDays(-1));
        redeemed.Redeem("auth0|other-user", fixedNow.AddMinutes(-5));
        await repo.CreateAsync(redeemed, CancellationToken.None).ConfigureAwait(false);

        await using var baseFactory = new TestWebApplicationFactory();
        await using var factory = baseFactory.WithWebHostBuilder(builder =>
            ConfigureOfferCodeHost(builder, repo, fixedNow));

        await SeedFreeUserAsync(factory, UserId).ConfigureAwait(false);
        using var client = CreateAuthenticatedClient(factory);

        var response = await client.PostAsJsonAsync(
            new Uri("/v1/offer-codes/redeem", UriKind.Relative),
            new RedeemOfferCodeRequest(DisplayCode),
            AppJsonSerializerContext.Default.RedeemOfferCodeRequest).ConfigureAwait(false);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.Conflict);
        var body = await response.Content.ReadFromJsonAsync(
            AppJsonSerializerContext.Default.ApiErrorResponse).ConfigureAwait(false);
        await Assert.That(body).IsNotNull();
        await Assert.That(body!.Error).IsEqualTo("code_already_redeemed");
        await Assert.That(body.Message).IsNotNull();
    }

    [Test]
    public async Task Should_Return409_WithAlreadySubscribed_When_UserIsNotFreeTier()
    {
        var fixedNow = new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero);
        var repo = new InMemoryOfferCodeRepository();
        await repo.CreateAsync(
            new OfferCode(CanonicalCode, SubscriptionTier.Pro, 30, fixedNow.AddDays(-1)),
            CancellationToken.None).ConfigureAwait(false);

        await using var baseFactory = new TestWebApplicationFactory();
        await using var factory = baseFactory.WithWebHostBuilder(builder =>
            ConfigureOfferCodeHost(builder, repo, fixedNow));

        var profileRepo = factory.Services.GetRequiredService<IUserProfileRepository>();
        var profile = UserProfile.Register(UserId, $"{UserId}@example.com");
        profile.ActivateSubscription(SubscriptionTier.Personal, fixedNow.AddDays(60));
        await profileRepo.SaveAsync(profile, CancellationToken.None).ConfigureAwait(false);

        using var client = CreateAuthenticatedClient(factory);

        var response = await client.PostAsJsonAsync(
            new Uri("/v1/offer-codes/redeem", UriKind.Relative),
            new RedeemOfferCodeRequest(DisplayCode),
            AppJsonSerializerContext.Default.RedeemOfferCodeRequest).ConfigureAwait(false);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.Conflict);
        var body = await response.Content.ReadFromJsonAsync(
            AppJsonSerializerContext.Default.ApiErrorResponse).ConfigureAwait(false);
        await Assert.That(body).IsNotNull();
        await Assert.That(body!.Error).IsEqualTo("already_subscribed");
        await Assert.That(body.Message).IsNotNull();
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
