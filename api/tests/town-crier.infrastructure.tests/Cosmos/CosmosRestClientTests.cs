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
