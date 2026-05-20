using TownCrier.Infrastructure.Subscriptions;

namespace TownCrier.Infrastructure.Tests.Subscriptions;

public sealed class TransactionDecoderTests
{
    private const string ValidJson =
        """
        {
          "transactionId": "txn-1",
          "originalTransactionId": "orig-txn-1",
          "productId": "uk.co.towncrier.personal.monthly",
          "bundleId": "uk.co.towncrier.ios",
          "purchaseDate": 1744329600000,
          "expiresDate": 1746921600000,
          "environment": "Production"
        }
        """;

    [Test]
    public async Task Should_MapAllFields_When_JsonIsWellFormed()
    {
        var decoder = new TransactionDecoder();

        var transaction = decoder.Decode(ValidJson);

        await Assert.That(transaction.TransactionId).IsEqualTo("txn-1");
        await Assert.That(transaction.OriginalTransactionId).IsEqualTo("orig-txn-1");
        await Assert.That(transaction.ProductId).IsEqualTo("uk.co.towncrier.personal.monthly");
        await Assert.That(transaction.BundleId).IsEqualTo("uk.co.towncrier.ios");
        await Assert.That(transaction.Environment).IsEqualTo("Production");
    }

    [Test]
    public async Task Should_ConvertUnixEpochMillisecondsToDateTimeOffset()
    {
        var decoder = new TransactionDecoder();

        var transaction = decoder.Decode(ValidJson);

        await Assert.That(transaction.PurchaseDate)
            .IsEqualTo(DateTimeOffset.FromUnixTimeMilliseconds(1744329600000));
        await Assert.That(transaction.ExpiresDate)
            .IsEqualTo(DateTimeOffset.FromUnixTimeMilliseconds(1746921600000));
    }

    [Test]
    public void Should_Throw_When_RequiredFieldIsMissing()
    {
        var decoder = new TransactionDecoder();
        const string missingProductId =
            """
            {
              "transactionId": "txn-1",
              "originalTransactionId": "orig-txn-1",
              "bundleId": "uk.co.towncrier.ios",
              "purchaseDate": 1744329600000,
              "expiresDate": 1746921600000,
              "environment": "Production"
            }
            """;

        Assert.Throws<ArgumentException>(() => decoder.Decode(missingProductId));
    }

    [Test]
    public void Should_Throw_When_JsonIsMalformed()
    {
        var decoder = new TransactionDecoder();

        Assert.Throws<ArgumentException>(() => decoder.Decode("{not json"));
    }
}
