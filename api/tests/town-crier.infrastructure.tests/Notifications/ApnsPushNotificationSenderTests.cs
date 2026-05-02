using System.Net;
using System.Net.Http.Headers;
using System.Text.Json;
using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Infrastructure.Notifications;
using TownCrier.Infrastructure.Tests.Cosmos;
using TownCrier.Infrastructure.Tests.Polling;

namespace TownCrier.Infrastructure.Tests.Notifications;

public sealed class ApnsPushNotificationSenderTests
{
    private const string TestKeyId = "ABC1234567";
    private const string TestTeamId = "DEF7654321";
    private const string TestBundleId = "uk.towncrierapp.mobile";

    [Test]
    public async Task Should_ReturnEmptyInvalidTokens_When_AllDevicesAccepted()
    {
        // Arrange
        var handler = new StubHttpHandler();
        handler.EnqueueResponse(HttpStatusCode.OK);
        using var sender = CreateSender(handler);
        var notification = new NotificationBuilder().Build();
        var devices = new List<DeviceRegistration>
        {
            DeviceRegistration.Create("user-1", "token-1", DevicePlatform.iOS, DateTimeOffset.UtcNow),
        };

        // Act
        var result = await sender.SendAsync(notification, devices, CancellationToken.None);

        // Assert
        await Assert.That(result.InvalidTokens).IsEmpty();
        await Assert.That(handler.SentRequests.Count).IsEqualTo(1);
    }

    private static ApnsPushNotificationSender CreateSender(HttpMessageHandler handler)
    {
        var pem = ApnsJwtProviderTestKey.GeneratePkcs8Pem();
        var time = new FakeTimeProvider();
        time.SetUtcNow(new DateTimeOffset(2026, 5, 2, 12, 0, 0, TimeSpan.Zero));
        var jwtProvider = new ApnsJwtProvider(pem, TestKeyId, TestTeamId, time);
        var options = new ApnsOptions
        {
            BundleId = TestBundleId,
            UseSandbox = true,
            MaxParallelism = 4,
        };
        var httpClient = new HttpClient(handler, disposeHandler: false)
        {
            BaseAddress = options.ResolveBaseAddress(),
        };
        return new ApnsPushNotificationSender(httpClient, jwtProvider, options, NullLogger<ApnsPushNotificationSender>.Instance, time);
    }
}
