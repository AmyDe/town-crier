using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Tests.Cosmos;

public sealed class FakeCosmosRestClientCasTests
{
    [Test]
    public async Task Create_AssignsEtag_And_ReadReturnsIt()
    {
        var fake = new FakeCosmosRestClient();
        var created = await fake.TryCreateDocumentAsync(
            "c",
            new TestDocument { Id = "a", Payload = "v0" },
            "a",
            TestSerializerContext.Default.TestDocument,
            default);
        await Assert.That(created).IsTrue();

        var read = await fake.ReadDocumentWithETagAsync(
            "c",
            "a",
            "a",
            TestSerializerContext.Default.TestDocument,
            default);
        await Assert.That(read.Document!.Payload).IsEqualTo("v0");
        await Assert.That(read.ETag).IsNotNull();
    }

    [Test]
    public async Task Create_SecondCallReturnsFalse_When_DocumentExists()
    {
        var fake = new FakeCosmosRestClient();
        await fake.TryCreateDocumentAsync(
            "c",
            new TestDocument { Id = "a", Payload = "v0" },
            "a",
            TestSerializerContext.Default.TestDocument,
            default);
        var secondCreate = await fake.TryCreateDocumentAsync(
            "c",
            new TestDocument { Id = "a", Payload = "v1" },
            "a",
            TestSerializerContext.Default.TestDocument,
            default);
        await Assert.That(secondCreate).IsFalse();
    }

    [Test]
    public async Task Replace_ReturnsTrue_WithMatchingEtag_AndBumpsEtag()
    {
        var fake = new FakeCosmosRestClient();
        await fake.TryCreateDocumentAsync(
            "c",
            new TestDocument { Id = "a", Payload = "v0" },
            "a",
            TestSerializerContext.Default.TestDocument,
            default);
        var read1 = await fake.ReadDocumentWithETagAsync(
            "c",
            "a",
            "a",
            TestSerializerContext.Default.TestDocument,
            default);

        var ok = await fake.TryReplaceDocumentAsync(
            "c",
            new TestDocument { Id = "a", Payload = "v1" },
            "a",
            read1.ETag!,
            TestSerializerContext.Default.TestDocument,
            default);
        await Assert.That(ok).IsTrue();

        var read2 = await fake.ReadDocumentWithETagAsync(
            "c",
            "a",
            "a",
            TestSerializerContext.Default.TestDocument,
            default);
        await Assert.That(read2.ETag).IsNotEqualTo(read1.ETag);
    }

    [Test]
    public async Task Replace_ReturnsFalse_WithStaleEtag()
    {
        var fake = new FakeCosmosRestClient();
        await fake.TryCreateDocumentAsync(
            "c",
            new TestDocument { Id = "a", Payload = "v0" },
            "a",
            TestSerializerContext.Default.TestDocument,
            default);

        var ok = await fake.TryReplaceDocumentAsync(
            "c",
            new TestDocument { Id = "a", Payload = "v1" },
            "a",
            "\"stale\"",
            TestSerializerContext.Default.TestDocument,
            default);
        await Assert.That(ok).IsFalse();
    }

    [Test]
    public async Task Delete_PreconditionFailed_WithStaleEtag()
    {
        var fake = new FakeCosmosRestClient();
        await fake.TryCreateDocumentAsync(
            "c",
            new TestDocument { Id = "a", Payload = "v0" },
            "a",
            TestSerializerContext.Default.TestDocument,
            default);

        var outcome = await fake.TryDeleteDocumentAsync("c", "a", "a", "\"stale\"", default);
        await Assert.That(outcome).IsEqualTo(CosmosDeleteOutcome.PreconditionFailed);
    }
}
