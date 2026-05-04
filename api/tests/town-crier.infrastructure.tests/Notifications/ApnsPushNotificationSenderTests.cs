using System.Net;
using System.Text.Json;
using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Infrastructure.Notifications;
using TownCrier.Infrastructure.Tests.Polling;

namespace TownCrier.Infrastructure.Tests.Notifications;

public sealed class ApnsPushNotificationSenderTests
{
    private const string TestKeyId = "ABC1234567";
    private const string TestTeamId = "DEF7654321";
    private const string TestBundleId = "uk.towncrierapp.mobile";

    private static readonly string[] DeadTokenList = ["dead-token"];
    private static readonly string[] GarbageTokenList = ["garbage-token"];
    private static readonly string[] StaleAndBadTokens = ["stale-token", "bad-token"];

    [Test]
    public async Task Should_ReturnEmptyInvalidTokens_When_AllDevicesAccepted()
    {
        // Arrange
        using var fixture = SenderFixture.Create();
        fixture.Handler.EnqueueOk();
        var notification = new NotificationBuilder().Build();
        var devices = OneDevice("token-1");

        // Act
        var result = await fixture.Sender.SendAsync(notification, devices, totalUnreadCount: 1, CancellationToken.None);

        // Assert
        await Assert.That(result.InvalidTokens).IsEmpty();
        await Assert.That(fixture.Handler.SentRequests.Count).IsEqualTo(1);
    }

    [Test]
    public async Task Should_PostToCorrectDeviceUrl_When_Sending()
    {
        // Arrange
        using var fixture = SenderFixture.Create();
        fixture.Handler.EnqueueOk();
        var notification = new NotificationBuilder().Build();
        var devices = OneDevice("abcdef0123456789");

        // Act
        await fixture.Sender.SendAsync(notification, devices, totalUnreadCount: 1, CancellationToken.None);

        // Assert
        var request = fixture.Handler.SentRequests[0];
        await Assert.That(request.Method).IsEqualTo(HttpMethod.Post);
        await Assert.That(request.RequestUri!.AbsolutePath).IsEqualTo("/3/device/abcdef0123456789");
    }

    [Test]
    public async Task Should_AttachJwtBearerAuthorizationHeader_When_Sending()
    {
        // Arrange
        using var fixture = SenderFixture.Create();
        fixture.Handler.EnqueueOk();
        var notification = new NotificationBuilder().Build();
        var devices = OneDevice("token-1");

        // Act
        await fixture.Sender.SendAsync(notification, devices, totalUnreadCount: 1, CancellationToken.None);

        // Assert
        var auth = fixture.Handler.SentRequests[0].Authorization;
        await Assert.That(auth).IsNotNull();
        await Assert.That(auth!.Scheme).IsEqualTo("bearer");
        await Assert.That(auth.Parameter).IsNotNull();
        await Assert.That(auth.Parameter!.Split('.').Length).IsEqualTo(3);
    }

    [Test]
    public async Task Should_SetApnsHeaders_When_SendingAlertNotification()
    {
        // Arrange
        using var fixture = SenderFixture.Create();
        fixture.Handler.EnqueueOk();
        var notification = new NotificationBuilder().Build();
        var devices = OneDevice("token-1");

        // Act
        await fixture.Sender.SendAsync(notification, devices, totalUnreadCount: 1, CancellationToken.None);

        // Assert
        var headers = fixture.Handler.SentRequests[0].Headers;
        await Assert.That(headers["apns-topic"]).IsEqualTo(TestBundleId);
        await Assert.That(headers["apns-push-type"]).IsEqualTo("alert");
        await Assert.That(headers["apns-priority"]).IsEqualTo("10");
        await Assert.That(headers["apns-expiration"]).IsEqualTo("0");
    }

    [Test]
    public async Task Should_SendHttp2Request_When_Posting()
    {
        // Arrange
        using var fixture = SenderFixture.Create();
        fixture.Handler.EnqueueOk();
        var notification = new NotificationBuilder().Build();
        var devices = OneDevice("token-1");

        // Act
        await fixture.Sender.SendAsync(notification, devices, totalUnreadCount: 1, CancellationToken.None);

        // Assert
        await Assert.That(fixture.Handler.SentRequests[0].Version).IsEqualTo(HttpVersion.Version20);
        await Assert.That(fixture.Handler.SentRequests[0].VersionPolicy).IsEqualTo(HttpVersionPolicy.RequestVersionOrHigher);
    }

    [Test]
    public async Task Should_BuildAlertPayloadWithTitleBodyAndCustomKeys_When_Sending()
    {
        // Arrange
        using var fixture = SenderFixture.Create();
        fixture.Handler.EnqueueOk();
        var notification = new NotificationBuilder()
            .WithApplicationName("APP/2026/0042")
            .WithApplicationAddress("12 High Street")
            .Build();
        var devices = OneDevice("token-1");

        // Act
        await fixture.Sender.SendAsync(notification, devices, totalUnreadCount: 1, CancellationToken.None);

        // Assert
        var bodyJson = fixture.Handler.SentRequests[0].Body;
        using var doc = JsonDocument.Parse(bodyJson);
        var root = doc.RootElement;
        var aps = root.GetProperty("aps");
        var alert = aps.GetProperty("alert");
        await Assert.That(alert.GetProperty("title").GetString()).IsNotNull();
        await Assert.That(alert.GetProperty("body").GetString()).IsNotNull();
        await Assert.That(aps.GetProperty("sound").GetString()).IsEqualTo("default");
        await Assert.That(root.GetProperty("notificationId").GetString()).IsEqualTo(notification.Id);
        await Assert.That(root.GetProperty("applicationRef").GetString()).IsEqualTo("APP/2026/0042");
    }

    [Test]
    public async Task Should_AddTokenToInvalidTokens_When_ApnsReturns410Unregistered()
    {
        // Arrange
        using var fixture = SenderFixture.Create();
        fixture.Handler.EnqueueRejection(HttpStatusCode.Gone, "Unregistered");
        var notification = new NotificationBuilder().Build();
        var devices = OneDevice("dead-token");

        // Act
        var result = await fixture.Sender.SendAsync(notification, devices, totalUnreadCount: 1, CancellationToken.None);

        // Assert
        await Assert.That(result.InvalidTokens).IsEquivalentTo(DeadTokenList);
    }

    [Test]
    public async Task Should_AddTokenToInvalidTokens_When_ApnsReturns400BadDeviceToken()
    {
        // Arrange
        using var fixture = SenderFixture.Create();
        fixture.Handler.EnqueueRejection(HttpStatusCode.BadRequest, "BadDeviceToken");
        var notification = new NotificationBuilder().Build();
        var devices = OneDevice("garbage-token");

        // Act
        var result = await fixture.Sender.SendAsync(notification, devices, totalUnreadCount: 1, CancellationToken.None);

        // Assert
        await Assert.That(result.InvalidTokens).IsEquivalentTo(GarbageTokenList);
    }

    [Test]
    public async Task Should_NotAddTokenToInvalidTokens_When_BadRequestReasonIsNotBadDeviceToken()
    {
        // Arrange — APNs sometimes returns 400 for other reasons (PayloadEmpty, BadTopic) that
        // do not mean the device token itself is invalid; the sender must not prune those.
        using var fixture = SenderFixture.Create();
        fixture.Handler.EnqueueRejection(HttpStatusCode.BadRequest, "PayloadEmpty");
        var notification = new NotificationBuilder().Build();
        var devices = OneDevice("token-1");

        // Act
        var result = await fixture.Sender.SendAsync(notification, devices, totalUnreadCount: 1, CancellationToken.None);

        // Assert
        await Assert.That(result.InvalidTokens).IsEmpty();
    }

    [Test]
    public async Task Should_RetryOnce_When_ApnsReturns403ExpiredProviderToken()
    {
        // Arrange — first call gets 403 ExpiredProviderToken, second succeeds.
        using var fixture = SenderFixture.Create();
        fixture.Handler.EnqueueRejection(HttpStatusCode.Forbidden, "ExpiredProviderToken");
        fixture.Handler.EnqueueOk();
        var notification = new NotificationBuilder().Build();
        var devices = OneDevice("token-1");

        // Act
        var result = await fixture.Sender.SendAsync(notification, devices, totalUnreadCount: 1, CancellationToken.None);

        // Assert
        await Assert.That(result.InvalidTokens).IsEmpty();
        await Assert.That(fixture.Handler.SentRequests.Count).IsEqualTo(2);
    }

    [Test]
    public async Task Should_MintFreshJwt_When_RetryingAfter403ExpiredProviderToken()
    {
        // Arrange — the second attempt must use a different JWT than the first.
        using var fixture = SenderFixture.Create();
        fixture.Handler.EnqueueRejection(HttpStatusCode.Forbidden, "ExpiredProviderToken");
        fixture.Handler.EnqueueOk();
        var notification = new NotificationBuilder().Build();
        var devices = OneDevice("token-1");

        // Advance the clock between the two attempts so the freshly-minted JWT carries a new iat.
        // The sender mints lazily on each request; advancing time forces a new mint after Invalidate().
        fixture.TimeProvider.Advance(TimeSpan.FromSeconds(1));

        // Act
        await fixture.Sender.SendAsync(notification, devices, totalUnreadCount: 1, CancellationToken.None);

        // Assert
        var firstJwt = fixture.Handler.SentRequests[0].Authorization!.Parameter;
        var secondJwt = fixture.Handler.SentRequests[1].Authorization!.Parameter;
        await Assert.That(secondJwt).IsNotEqualTo(firstJwt);
    }

    [Test]
    public async Task Should_ReuseCachedJwt_When_NotInvalidated()
    {
        // Arrange — without a 403 there is no Invalidate(), so the second request should
        // reuse the cached JWT (provided we don't cross the 50-minute refresh boundary).
        using var fixture = SenderFixture.Create();
        fixture.Handler.EnqueueOk();
        fixture.Handler.EnqueueOk();
        var notification = new NotificationBuilder().Build();
        var devices = new List<DeviceRegistration>
        {
            DeviceRegistration.Create("user-1", "token-1", DevicePlatform.Ios, DateTimeOffset.UtcNow),
            DeviceRegistration.Create("user-2", "token-2", DevicePlatform.Ios, DateTimeOffset.UtcNow),
        };

        // Act
        await fixture.Sender.SendAsync(notification, devices, totalUnreadCount: 1, CancellationToken.None);

        // Assert
        var firstJwt = fixture.Handler.SentRequests[0].Authorization!.Parameter;
        var secondJwt = fixture.Handler.SentRequests[1].Authorization!.Parameter;
        await Assert.That(secondJwt).IsEqualTo(firstJwt);
    }

    [Test]
    public async Task Should_GiveUp_When_403ExpiredProviderTokenReturnedTwice()
    {
        // Arrange — both attempts return 403; do not loop forever, do not prune token.
        using var fixture = SenderFixture.Create();
        fixture.Handler.EnqueueRejection(HttpStatusCode.Forbidden, "ExpiredProviderToken");
        fixture.Handler.EnqueueRejection(HttpStatusCode.Forbidden, "ExpiredProviderToken");
        var notification = new NotificationBuilder().Build();
        var devices = OneDevice("token-1");

        // Act
        var result = await fixture.Sender.SendAsync(notification, devices, totalUnreadCount: 1, CancellationToken.None);

        // Assert — token is not invalidated; we tried twice and stopped.
        await Assert.That(result.InvalidTokens).IsEmpty();
        await Assert.That(fixture.Handler.SentRequests.Count).IsEqualTo(2);
    }

    [Test]
    public async Task Should_NotPruneToken_When_ApnsReturns429TooManyProviderTokenUpdates()
    {
        // Arrange — a 429 is a transient JWT-mint problem, not a bad token.
        using var fixture = SenderFixture.Create();
        fixture.Handler.EnqueueRejection(HttpStatusCode.TooManyRequests, "TooManyProviderTokenUpdates");
        var notification = new NotificationBuilder().Build();
        var devices = OneDevice("token-1");

        // Act
        var result = await fixture.Sender.SendAsync(notification, devices, totalUnreadCount: 1, CancellationToken.None);

        // Assert
        await Assert.That(result.InvalidTokens).IsEmpty();
    }

    [Test]
    public async Task Should_RetryUpToThreeTimes_When_ApnsReturns5xx()
    {
        // Arrange — three 500s in a row, sender gives up without pruning.
        using var fixture = SenderFixture.Create();
        fixture.Handler.EnqueueStatus(HttpStatusCode.InternalServerError);
        fixture.Handler.EnqueueStatus(HttpStatusCode.InternalServerError);
        fixture.Handler.EnqueueStatus(HttpStatusCode.InternalServerError);
        var notification = new NotificationBuilder().Build();
        var devices = OneDevice("token-1");

        // Act
        var result = await fixture.Sender.SendAsync(notification, devices, totalUnreadCount: 1, CancellationToken.None);

        // Assert
        await Assert.That(result.InvalidTokens).IsEmpty();
        await Assert.That(fixture.Handler.SentRequests.Count).IsEqualTo(3);
    }

    [Test]
    public async Task Should_SucceedWithoutPruning_When_5xxRecoversBeforeMaxAttempts()
    {
        // Arrange — first attempt 503, second attempt 200.
        using var fixture = SenderFixture.Create();
        fixture.Handler.EnqueueStatus(HttpStatusCode.ServiceUnavailable);
        fixture.Handler.EnqueueOk();
        var notification = new NotificationBuilder().Build();
        var devices = OneDevice("token-1");

        // Act
        var result = await fixture.Sender.SendAsync(notification, devices, totalUnreadCount: 1, CancellationToken.None);

        // Assert
        await Assert.That(result.InvalidTokens).IsEmpty();
        await Assert.That(fixture.Handler.SentRequests.Count).IsEqualTo(2);
    }

    [Test]
    public async Task Should_NotExceedMaxParallelism_When_SendingToManyDevices()
    {
        // Arrange — 20 devices, parallelism cap of 3. Hold each response open briefly so
        // many requests pile up in flight, then verify the in-flight count never exceeded the cap.
        using var fixture = SenderFixture.Create(maxParallelism: 3);
        fixture.Handler.ResponseDelay = TimeSpan.FromMilliseconds(20);
        for (var i = 0; i < 20; i++)
        {
            fixture.Handler.EnqueueOk();
        }

        var notification = new NotificationBuilder().Build();
        var devices = Enumerable.Range(0, 20)
            .Select(i => DeviceRegistration.Create($"user-{i}", $"token-{i}", DevicePlatform.Ios, DateTimeOffset.UtcNow))
            .ToList();

        // Act
        var result = await fixture.Sender.SendAsync(notification, devices, totalUnreadCount: 1, CancellationToken.None);

        // Assert
        await Assert.That(result.InvalidTokens).IsEmpty();
        await Assert.That(fixture.Handler.SentRequests.Count).IsEqualTo(20);
        await Assert.That(fixture.Handler.PeakConcurrency).IsLessThanOrEqualTo(3);
    }

    [Test]
    public async Task Should_BuildDigestPayloadWithCount_When_SendingDigest()
    {
        // Arrange — applicationCount drives the human-readable body copy ("7 new
        // applications…") while totalUnreadCount drives the iOS badge.
        using var fixture = SenderFixture.Create();
        fixture.Handler.EnqueueOk();
        var devices = OneDevice("token-1");

        // Act
        var result = await fixture.Sender.SendDigestAsync(
            applicationCount: 7, totalUnreadCount: 12, devices, CancellationToken.None);

        // Assert
        await Assert.That(result.InvalidTokens).IsEmpty();
        var bodyJson = fixture.Handler.SentRequests[0].Body;
        using var doc = JsonDocument.Parse(bodyJson);
        var aps = doc.RootElement.GetProperty("aps");
        await Assert.That(aps.GetProperty("alert").GetProperty("title").GetString()).IsEqualTo("Town Crier");
        await Assert.That(aps.GetProperty("alert").GetProperty("body").GetString()!).Contains("7");
        await Assert.That(aps.GetProperty("badge").GetInt32()).IsEqualTo(12);
    }

    [Test]
    public async Task Should_SetAlertBadgeToTotalUnreadCount_When_Sending()
    {
        // Arrange — pin the new contract: the alert payload's badge is the caller's
        // totalUnreadCount, not a hardcoded 1. Per spec this drives the iOS app
        // icon badge so post-mark-all-read pushes can decrement back to 0.
        using var fixture = SenderFixture.Create();
        fixture.Handler.EnqueueOk();
        var notification = new NotificationBuilder().Build();
        var devices = OneDevice("token-1");

        // Act
        await fixture.Sender.SendAsync(notification, devices, totalUnreadCount: 5, CancellationToken.None);

        // Assert
        var bodyJson = fixture.Handler.SentRequests[0].Body;
        using var doc = JsonDocument.Parse(bodyJson);
        await Assert.That(doc.RootElement.GetProperty("aps").GetProperty("badge").GetInt32()).IsEqualTo(5);
    }

    [Test]
    public async Task Should_ReturnEmpty_When_DeviceListIsEmpty()
    {
        // Arrange
        using var fixture = SenderFixture.Create();
        var notification = new NotificationBuilder().Build();

        // Act
        var result = await fixture.Sender.SendAsync(notification, [], totalUnreadCount: 0, CancellationToken.None);

        // Assert — nothing sent, nothing rejected.
        await Assert.That(result.InvalidTokens).IsEmpty();
        await Assert.That(fixture.Handler.SentRequests.Count).IsEqualTo(0);
    }

    [Test]
    public async Task Should_ReportInvalidTokensFromMixedResponses_When_SomeFail()
    {
        // Arrange — three devices: 1 OK, 1 410 Unregistered, 1 400 BadDeviceToken.
        using var fixture = SenderFixture.Create(maxParallelism: 1);
        fixture.Handler.EnqueueOk();
        fixture.Handler.EnqueueRejection(HttpStatusCode.Gone, "Unregistered");
        fixture.Handler.EnqueueRejection(HttpStatusCode.BadRequest, "BadDeviceToken");
        var notification = new NotificationBuilder().Build();
        var devices = new List<DeviceRegistration>
        {
            DeviceRegistration.Create("user-1", "ok-token", DevicePlatform.Ios, DateTimeOffset.UtcNow),
            DeviceRegistration.Create("user-2", "stale-token", DevicePlatform.Ios, DateTimeOffset.UtcNow),
            DeviceRegistration.Create("user-3", "bad-token", DevicePlatform.Ios, DateTimeOffset.UtcNow),
        };

        // Act
        var result = await fixture.Sender.SendAsync(notification, devices, totalUnreadCount: 1, CancellationToken.None);

        // Assert — both rejected tokens reported; OK token is not.
        await Assert.That(result.InvalidTokens).IsEquivalentTo(StaleAndBadTokens);
    }

    private static List<DeviceRegistration> OneDevice(string token) =>
        [DeviceRegistration.Create("user-1", token, DevicePlatform.Ios, DateTimeOffset.UtcNow)];

    private sealed class SenderFixture : IDisposable
    {
        private SenderFixture(
            FakeApnsHandler handler,
            HttpClient client,
            ApnsJwtProvider jwt,
            ApnsPushNotificationSender sender,
            FakeTimeProvider timeProvider)
        {
            this.Handler = handler;
            this.Client = client;
            this.JwtProvider = jwt;
            this.Sender = sender;
            this.TimeProvider = timeProvider;
        }

        public FakeApnsHandler Handler { get; }

        public HttpClient Client { get; }

        public ApnsJwtProvider JwtProvider { get; }

        public ApnsPushNotificationSender Sender { get; }

        public FakeTimeProvider TimeProvider { get; }

        public static SenderFixture Create(int maxParallelism = 4)
        {
            var handler = new FakeApnsHandler();
            var pem = ApnsJwtProviderTestKey.GeneratePkcs8Pem();
            var time = new FakeTimeProvider();
            time.SetUtcNow(new DateTimeOffset(2026, 5, 2, 12, 0, 0, TimeSpan.Zero));
            var jwt = new ApnsJwtProvider(pem, TestKeyId, TestTeamId, time);
            var options = new ApnsOptions
            {
                BundleId = TestBundleId,
                UseSandbox = true,
                MaxParallelism = maxParallelism,
            };
            var client = new HttpClient(handler, disposeHandler: false)
            {
                BaseAddress = options.ResolveBaseAddress(),
            };
            var sender = new ApnsPushNotificationSender(client, jwt, options, NullLogger<ApnsPushNotificationSender>.Instance, time);
            return new SenderFixture(handler, client, jwt, sender, time);
        }

        public void Dispose()
        {
            this.Client.Dispose();
            this.JwtProvider.Dispose();
            this.Handler.Dispose();
        }
    }
}
