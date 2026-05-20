using System.Net;
using System.Net.Http.Headers;
using System.Net.Http.Json;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Mvc.Testing;
using Microsoft.AspNetCore.TestHost;
using Microsoft.Extensions.DependencyInjection;
using TownCrier.Application.Subscriptions;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;
using TownCrier.Infrastructure.UserProfiles;
using TownCrier.Web.Endpoints;
using TownCrier.Web.Tests.Auth;

namespace TownCrier.Web.Tests.Subscriptions;

public sealed class VerifySubscriptionEndpointTests
{
    private const string UserId = "auth0|test-user-123";
    private const string BundleId = "uk.co.towncrier.ios";

    private const string PersonalTransactionJson =
        $$"""
        {
          "transactionId": "txn-1",
          "originalTransactionId": "orig-txn-1",
          "productId": "uk.co.towncrier.personal.monthly",
          "bundleId": "{{BundleId}}",
          "purchaseDate": 1744329600000,
          "expiresDate": 1746921600000,
          "environment": "Sandbox"
        }
        """;

    private static readonly Uri VerifyUri = new("/v1/subscriptions/verify", UriKind.Relative);

    [Test]
    public async Task Should_Return200WithEntitlementState_When_TransactionIsValid()
    {
        await using var baseFactory = new TestWebApplicationFactory();
        await using var factory = baseFactory.WithWebHostBuilder(
            builder => ConfigureSubscriptionHost(builder, StubAppleJwsVerifier.ReturningPayload(PersonalTransactionJson)));
        using var client = AuthenticatedClient(factory);

        var response = await client.PostAsJsonAsync(
            VerifyUri,
            new VerifySubscriptionRequest("header.payload.signature"),
            AppJsonSerializerContext.Default.VerifySubscriptionRequest).ConfigureAwait(false);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);

        var body = await response.Content
            .ReadFromJsonAsync(AppJsonSerializerContext.Default.VerifySubscriptionResponse)
            .ConfigureAwait(false);
        await Assert.That(body!.Tier).IsEqualTo("Personal");
        await Assert.That(body.WatchZoneLimit).IsEqualTo(3);
        await Assert.That(body.Entitlements).Contains("StatusChangeAlerts");
    }

    [Test]
    public async Task Should_Return401_When_JwsSignatureIsInvalid()
    {
        await using var baseFactory = new TestWebApplicationFactory();
        await using var factory = baseFactory.WithWebHostBuilder(
            builder => ConfigureSubscriptionHost(builder, StubAppleJwsVerifier.ThatRejects()));
        using var client = AuthenticatedClient(factory);

        var response = await client.PostAsJsonAsync(
            VerifyUri,
            new VerifySubscriptionRequest("tampered.jws.signature"),
            AppJsonSerializerContext.Default.VerifySubscriptionRequest).ConfigureAwait(false);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.Unauthorized);
    }

    [Test]
    public async Task Should_Return400_When_PayloadIsMalformed()
    {
        await using var baseFactory = new TestWebApplicationFactory();
        await using var factory = baseFactory.WithWebHostBuilder(
            builder => ConfigureSubscriptionHost(builder, StubAppleJwsVerifier.ReturningPayload(PersonalTransactionJson)));
        using var client = AuthenticatedClient(factory);

        using var content = new StringContent("{not json", System.Text.Encoding.UTF8, "application/json");
        var response = await client.PostAsync(VerifyUri, content).ConfigureAwait(false);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.BadRequest);
    }

    [Test]
    public async Task Should_Return401_When_RequestIsUnauthenticated()
    {
        await using var baseFactory = new TestWebApplicationFactory();
        await using var factory = baseFactory.WithWebHostBuilder(
            builder => ConfigureSubscriptionHost(builder, StubAppleJwsVerifier.ReturningPayload(PersonalTransactionJson)));
        using var client = factory.CreateClient();

        var response = await client.PostAsJsonAsync(
            VerifyUri,
            new VerifySubscriptionRequest("header.payload.signature"),
            AppJsonSerializerContext.Default.VerifySubscriptionRequest).ConfigureAwait(false);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.Unauthorized);
    }

    private static HttpClient AuthenticatedClient(WebApplicationFactory<Program> factory)
    {
        var client = factory.CreateClient();
        client.DefaultRequestHeaders.Authorization =
            new AuthenticationHeaderValue("Bearer", TestJwtToken.Generate(UserId));
        return client;
    }

    private static void ConfigureSubscriptionHost(
        IWebHostBuilder builder, IAppleJwsVerifier verifier)
    {
        builder.UseSetting("Apple:BundleId", BundleId);
        builder.UseSetting("Apple:Environment", "Sandbox");

        builder.ConfigureTestServices(services =>
        {
            services.AddSingleton(verifier);

            var repository = new InMemoryUserProfileRepository();
            repository.SaveAsync(UserProfile.Register(UserId), CancellationToken.None)
                .GetAwaiter().GetResult();
            services.AddSingleton<IUserProfileRepository>(repository);
        });
    }
}
