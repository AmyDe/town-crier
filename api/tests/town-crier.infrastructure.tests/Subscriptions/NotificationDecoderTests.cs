using TownCrier.Infrastructure.Subscriptions;

namespace TownCrier.Infrastructure.Tests.Subscriptions;

public sealed class NotificationDecoderTests
{
    // The shape of an App Store Server Notification v2 responseBodyV2DecodedPayload:
    // notificationType / subtype / notificationUUID at the top level, with the
    // signed JWS strings nested under "data".
    private const string ValidJson =
        """
        {
          "notificationType": "DID_RENEW",
          "subtype": "BILLING_RECOVERY",
          "notificationUUID": "9ad9eb1c-1f0b-4e21-9d2e-2f1f8c8e0001",
          "version": "2.0",
          "data": {
            "appAppleId": 1234567890,
            "bundleId": "uk.co.towncrier.ios",
            "environment": "Production",
            "signedTransactionInfo": "inner.txn.signature",
            "signedRenewalInfo": "inner.renewal.signature"
          }
        }
        """;

    [Test]
    public async Task Should_MapTopLevelFields_When_JsonIsWellFormed()
    {
        var decoder = new NotificationDecoder();

        var notification = decoder.Decode(ValidJson);

        await Assert.That(notification.NotificationType).IsEqualTo("DID_RENEW");
        await Assert.That(notification.Subtype).IsEqualTo("BILLING_RECOVERY");
        await Assert.That(notification.NotificationUuid)
            .IsEqualTo("9ad9eb1c-1f0b-4e21-9d2e-2f1f8c8e0001");
    }

    [Test]
    public async Task Should_MapNestedSignedJwsStrings_When_JsonIsWellFormed()
    {
        var decoder = new NotificationDecoder();

        var notification = decoder.Decode(ValidJson);

        await Assert.That(notification.SignedTransactionInfo).IsEqualTo("inner.txn.signature");
        await Assert.That(notification.SignedRenewalInfo).IsEqualTo("inner.renewal.signature");
    }

    [Test]
    public async Task Should_AllowNullSubtypeAndRenewalInfo_When_AbsentFromPayload()
    {
        var decoder = new NotificationDecoder();
        const string minimalJson =
            """
            {
              "notificationType": "EXPIRED",
              "notificationUUID": "9ad9eb1c-1f0b-4e21-9d2e-2f1f8c8e0002",
              "data": {
                "signedTransactionInfo": "inner.txn.signature"
              }
            }
            """;

        var notification = decoder.Decode(minimalJson);

        await Assert.That(notification.NotificationType).IsEqualTo("EXPIRED");
        await Assert.That(notification.Subtype).IsNull();
        await Assert.That(notification.SignedRenewalInfo).IsNull();
        await Assert.That(notification.SignedTransactionInfo).IsEqualTo("inner.txn.signature");
    }

    [Test]
    public void Should_Throw_When_NotificationTypeIsMissing()
    {
        var decoder = new NotificationDecoder();
        const string missingType =
            """
            {
              "notificationUUID": "9ad9eb1c-1f0b-4e21-9d2e-2f1f8c8e0003",
              "data": { "signedTransactionInfo": "inner.txn.signature" }
            }
            """;

        Assert.Throws<ArgumentException>(() => decoder.Decode(missingType));
    }

    [Test]
    public void Should_Throw_When_SignedTransactionInfoIsMissing()
    {
        var decoder = new NotificationDecoder();
        const string missingTxn =
            """
            {
              "notificationType": "DID_RENEW",
              "notificationUUID": "9ad9eb1c-1f0b-4e21-9d2e-2f1f8c8e0004",
              "data": {}
            }
            """;

        Assert.Throws<ArgumentException>(() => decoder.Decode(missingTxn));
    }

    [Test]
    public void Should_Throw_When_JsonIsMalformed()
    {
        var decoder = new NotificationDecoder();

        Assert.Throws<ArgumentException>(() => decoder.Decode("{not json"));
    }
}
