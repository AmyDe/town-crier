using System.Diagnostics.CodeAnalysis;
using System.Net;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Tests.Cosmos;

[SuppressMessage("Reliability", "CA2000:Dispose objects before losing scope", Justification = "HttpClient lifetime managed by test")]
public sealed class CosmosRestClientTests
{
    private const string AccountEndpoint = "https://test-account.documents.azure.com:443";
    private const string DatabaseName = "test-db";

    [Test]
    public async Task Should_BuildCorrectUrl_When_ReadingDocument()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.OK, """{"id":"doc1","name":"Test"}""");

        await client.ReadDocumentAsync(
            "Users",
            "doc1",
            "doc1",
            TestSerializerContext.Default.TestDocument,
            CancellationToken.None);

        var request = handler.SentRequests[0];
        await Assert.That(request.RequestUri!.AbsolutePath)
            .IsEqualTo("/dbs/test-db/colls/Users/docs/doc1");
        await Assert.That(request.Method).IsEqualTo(HttpMethod.Get);
    }

    [Test]
    public async Task Should_SetRequiredHeaders_When_ReadingDocument()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.OK, """{"id":"doc1","name":"Test"}""");

        await client.ReadDocumentAsync(
            "Users",
            "doc1",
            "pk1",
            TestSerializerContext.Default.TestDocument,
            CancellationToken.None);

        var request = handler.SentRequests[0];
        await Assert.That(request.Headers.GetValues("x-ms-version").First())
            .IsEqualTo("2018-12-31");
        await Assert.That(request.Headers.Contains("x-ms-date")).IsTrue();
        await Assert.That(request.Headers.Contains("Authorization")).IsTrue();
        await Assert.That(request.Headers.GetValues("x-ms-documentdb-partitionkey").First())
            .IsEqualTo("[\"pk1\"]");
    }

    [Test]
    public async Task Should_ReturnNull_When_ReadReturns404()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.NotFound);

        var result = await client.ReadDocumentAsync(
            "Users",
            "doc1",
            "doc1",
            TestSerializerContext.Default.TestDocument,
            CancellationToken.None);

        await Assert.That(result).IsNull();
    }

    [Test]
    public async Task Should_ThrowHttpRequestException_When_NonRetryableError()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.BadRequest);

        var exception = await Assert.ThrowsAsync<HttpRequestException>(async () =>
            await client.ReadDocumentAsync(
                "Users",
                "doc1",
                "doc1",
                TestSerializerContext.Default.TestDocument,
                CancellationToken.None));

        await Assert.That(exception).IsNotNull();
    }

    private static (CosmosRestClient Client, StubHttpHandler Handler) CreateClient()
    {
        var handler = new StubHttpHandler();
        var httpClient = new HttpClient(handler) { BaseAddress = new Uri(AccountEndpoint) };
        var credential = new StubTokenCredential("fake-token");
        var authProvider = new CosmosAuthProvider(credential);
        var options = new CosmosRestOptions
        {
            AccountEndpoint = AccountEndpoint,
            DatabaseName = DatabaseName,
        };
        var client = new CosmosRestClient(httpClient, authProvider, options);
        return (client, handler);
    }
}
