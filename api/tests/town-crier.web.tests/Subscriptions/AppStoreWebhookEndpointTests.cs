using System.Net;
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

namespace TownCrier.Web.Tests.Subscriptions;

public sealed class AppStoreWebhookEndpointTests
{
    private const string UserId = "auth0|test-user-123";
    private const string OriginalTransactionId = "orig-txn-1";
    private const string OuterJws = "outer.notification.jws";
    private const string InnerTransactionJws = "inner.transaction.jws";

    private const string ExpiredNotificationJson =
        $$"""
        {
          "notificationType": "EXPIRED",
          "notificationUUID": "9ad9eb1c-1f0b-4e21-9d2e-2f1f8c8e0001",
          "data": {
            "signedTransactionInfo": "{{InnerTransactionJws}}"
          }
        }
        """;

    private const string TransactionJson =
        $$"""
        {
          "transactionId": "txn-1",
          "originalTransactionId": "{{OriginalTransactionId}}",
          "productId": "uk.co.towncrier.personal.monthly",
          "bundleId": "uk.co.towncrier.ios",
          "purchaseDate": 1744329600000,
          "expiresDate": 1746921600000,
          "environment": "Sandbox"
        }
        """;

    private static readonly Uri WebhookUri = new("/v1/webhooks/appstore", UriKind.Relative);

    [Test]
    public async Task Should_Return200AndRevertTierToFree_When_ExpiredNotificationIsValid()
    {
        var repository = SubscribedUserRepository();
        var verifier = MappingAppleJwsVerifier.Create()
            .WithPayload(OuterJws, ExpiredNotificationJson)
            .WithPayload(InnerTransactionJws, TransactionJson);

        await using var baseFactory = new TestWebApplicationFactory();
        await using var factory = baseFactory.WithWebHostBuilder(
            builder => ConfigureWebhookHost(builder, verifier, repository));
        using var client = factory.CreateClient();

        var response = await client.PostAsJsonAsync(
            WebhookUri,
            new AppStoreNotificationRequest(OuterJws),
            AppJsonSerializerContext.Default.AppStoreNotificationRequest).ConfigureAwait(false);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);

        var profile = repository.GetByUserIdAsync(UserId, CancellationToken.None).GetAwaiter().GetResult();
        await Assert.That(profile!.Tier).IsEqualTo(SubscriptionTier.Free);
    }

    [Test]
    public async Task Should_Return401_When_JwsSignatureIsInvalid()
    {
        await using var baseFactory = new TestWebApplicationFactory();
        await using var factory = baseFactory.WithWebHostBuilder(
            builder => ConfigureWebhookHost(
                builder, MappingAppleJwsVerifier.ThatRejects(), SubscribedUserRepository()));
        using var client = factory.CreateClient();

        var response = await client.PostAsJsonAsync(
            WebhookUri,
            new AppStoreNotificationRequest("tampered.jws.payload"),
            AppJsonSerializerContext.Default.AppStoreNotificationRequest).ConfigureAwait(false);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.Unauthorized);
    }

    [Test]
    public async Task Should_Return400_When_PayloadIsMalformed()
    {
        await using var baseFactory = new TestWebApplicationFactory();
        await using var factory = baseFactory.WithWebHostBuilder(
            builder => ConfigureWebhookHost(
                builder, MappingAppleJwsVerifier.Create(), SubscribedUserRepository()));
        using var client = factory.CreateClient();

        using var content = new StringContent(
            "{not json", System.Text.Encoding.UTF8, "application/json");
        var response = await client.PostAsync(WebhookUri, content).ConfigureAwait(false);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.BadRequest);
    }

    [Test]
    public async Task Should_BeIdempotent_When_SameNotificationDeliveredTwice()
    {
        var repository = SubscribedUserRepository();
        var verifier = MappingAppleJwsVerifier.Create()
            .WithPayload(OuterJws, ExpiredNotificationJson)
            .WithPayload(InnerTransactionJws, TransactionJson);

        await using var baseFactory = new TestWebApplicationFactory();
        await using var factory = baseFactory.WithWebHostBuilder(
            builder => ConfigureWebhookHost(builder, verifier, repository));
        using var client = factory.CreateClient();

        var first = await client.PostAsJsonAsync(
            WebhookUri,
            new AppStoreNotificationRequest(OuterJws),
            AppJsonSerializerContext.Default.AppStoreNotificationRequest).ConfigureAwait(false);
        var second = await client.PostAsJsonAsync(
            WebhookUri,
            new AppStoreNotificationRequest(OuterJws),
            AppJsonSerializerContext.Default.AppStoreNotificationRequest).ConfigureAwait(false);

        await Assert.That(first.StatusCode).IsEqualTo(HttpStatusCode.OK);
        await Assert.That(second.StatusCode).IsEqualTo(HttpStatusCode.OK);

        var profile = repository.GetByUserIdAsync(UserId, CancellationToken.None).GetAwaiter().GetResult();
        await Assert.That(profile!.Tier).IsEqualTo(SubscriptionTier.Free);
    }

    private static InMemoryUserProfileRepository SubscribedUserRepository()
    {
        var profile = UserProfile.Register(UserId);
        profile.LinkOriginalTransactionId(OriginalTransactionId);
        profile.ActivateSubscription(
            SubscriptionTier.Personal,
            new DateTimeOffset(2026, 5, 11, 0, 0, 0, TimeSpan.Zero));

        var repository = new InMemoryUserProfileRepository();
        repository.SaveAsync(profile, CancellationToken.None).GetAwaiter().GetResult();
        return repository;
    }

    private static void ConfigureWebhookHost(
        IWebHostBuilder builder,
        IAppleJwsVerifier verifier,
        InMemoryUserProfileRepository repository)
    {
        builder.UseSetting("Apple:BundleId", "uk.co.towncrier.ios");
        builder.UseSetting("Apple:Environment", "Sandbox");

        builder.ConfigureTestServices(services =>
        {
            services.AddSingleton(verifier);
            services.AddSingleton<IUserProfileRepository>(repository);
            services.AddSingleton<INotificationIdempotencyStore, InMemoryNotificationIdempotencyStore>();
        });
    }
}
