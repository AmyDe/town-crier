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

    [Test]
    public async Task Should_SetUpsertHeader_When_UpsertingDocument()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.OK);

        await client.UpsertDocumentAsync(
            "Users",
            new TestDocument { Id = "doc1", Name = "Test" },
            "doc1",
            TestSerializerContext.Default.TestDocument,
            CancellationToken.None);

        var request = handler.SentRequests[0];
        await Assert.That(request.Method).IsEqualTo(HttpMethod.Post);
        await Assert.That(request.RequestUri!.AbsolutePath)
            .IsEqualTo("/dbs/test-db/colls/Users/docs");
        await Assert.That(request.Headers.GetValues("x-ms-documentdb-is-upsert").First())
            .IsEqualTo("True");
    }

    [Test]
    public async Task Should_SilentlySucceed_When_DeleteReturns404()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.NotFound);

        // Should not throw
        await client.DeleteDocumentAsync("Users", "doc1", "doc1", CancellationToken.None);

        await Assert.That(handler.SentRequests[0].Method).IsEqualTo(HttpMethod.Delete);
        await Assert.That(handler.SentRequests[0].RequestUri!.AbsolutePath)
            .IsEqualTo("/dbs/test-db/colls/Users/docs/doc1");
    }

    [Test]
    public async Task Should_SetQueryHeaders_When_Querying()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.OK, """{"Documents":[],"_count":0}""");

        await client.QueryAsync(
            "Users",
            "SELECT * FROM c",
            null,
            "pk1",
            TestSerializerContext.Default.TestDocument,
            CancellationToken.None);

        var request = handler.SentRequests[0];
        await Assert.That(request.Method).IsEqualTo(HttpMethod.Post);
        await Assert.That(request.Headers.GetValues("x-ms-documentdb-isquery").First())
            .IsEqualTo("True");
        await Assert.That(request.Content!.Headers.ContentType!.ToString())
            .IsEqualTo("application/query+json");
    }

    [Test]
    public async Task Should_DrainContinuationPages_When_QueryHasContinuation()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(
            HttpStatusCode.OK,
            """{"Documents":[{"id":"d1","name":"A"}],"_count":1}""",
            [new("x-ms-continuation", "page2-token")]);
        handler.EnqueueResponse(
            HttpStatusCode.OK,
            """{"Documents":[{"id":"d2","name":"B"}],"_count":1}""");

        var results = await client.QueryAsync(
            "Users",
            "SELECT * FROM c",
            null,
            "pk1",
            TestSerializerContext.Default.TestDocument,
            CancellationToken.None);

        await Assert.That(results).HasCount().EqualTo(2);
        await Assert.That(results[0].Id).IsEqualTo("d1");
        await Assert.That(results[1].Id).IsEqualTo("d2");
    }

    [Test]
    public async Task Should_SetCrossPartitionHeader_When_NoPartitionKey()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.OK, """{"Documents":[],"_count":0}""");

        await client.QueryAsync(
            "Users",
            "SELECT * FROM c",
            null,
            null,
            TestSerializerContext.Default.TestDocument,
            CancellationToken.None);

        var request = handler.SentRequests[0];
        await Assert.That(request.Headers.Contains("x-ms-documentdb-partitionkey")).IsFalse();
        await Assert.That(
            request.Headers.GetValues("x-ms-documentdb-query-enablecrosspartition").First())
            .IsEqualTo("True");
    }

    [Test]
    public async Task Should_ReturnFirstValue_When_ScalarQuery()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.OK, """{"Documents":[42],"_count":1}""");

        var result = await client.ScalarQueryAsync(
            "Users",
            "SELECT VALUE COUNT(1) FROM c",
            null,
            "pk1",
            TestSerializerContext.Default.Int32,
            CancellationToken.None);

        await Assert.That(result).IsEqualTo(42);
    }

    [Test]
    public async Task Should_FanOutToPartitionRanges_When_GatewayReturnsPartitionedQueryInfo()
    {
        var (client, handler) = CreateClient();

        // Request 1: cross-partition query returns 400 with fan-out marker
        // (uses SELECT * to exercise the runtime probe path — DISTINCT/GROUP BY skip this)
        handler.EnqueueResponse(
            HttpStatusCode.BadRequest,
            """{"code":"BadRequest","message":"Cross partition query ... partitionedQueryExecutionInfoVersion"}""");

        // Request 2: GET pkranges returns two partition key ranges
        handler.EnqueueResponse(
            HttpStatusCode.OK,
            """{"PartitionKeyRanges":[{"id":"0"},{"id":"1"}],"_count":2}""");

        // Request 3: query range 0
        handler.EnqueueResponse(
            HttpStatusCode.OK,
            """{"Documents":[{"id":"d1","name":"Alpha"}],"_count":1}""");

        // Request 4: query range 1
        handler.EnqueueResponse(
            HttpStatusCode.OK,
            """{"Documents":[{"id":"d2","name":"Beta"}],"_count":1}""");

        var results = await client.QueryAsync(
            "Users",
            "SELECT * FROM c WHERE ST_DISTANCE(c.location, @point) < 1000",
            null,
            null,
            TestSerializerContext.Default.TestDocument,
            CancellationToken.None);

        // Should have combined results from both ranges
        await Assert.That(results).HasCount().EqualTo(2);
        await Assert.That(results[0].Name).IsEqualTo("Alpha");
        await Assert.That(results[1].Name).IsEqualTo("Beta");

        // Should have sent 4 requests total
        await Assert.That(handler.SentRequests).HasCount().EqualTo(4);

        // Request 2 should be a GET to the pkranges endpoint
        var pkRangesRequest = handler.SentRequests[1];
        await Assert.That(pkRangesRequest.Method).IsEqualTo(HttpMethod.Get);
        await Assert.That(pkRangesRequest.RequestUri!.AbsolutePath)
            .IsEqualTo("/dbs/test-db/colls/Users/pkranges");

        // Requests 3 and 4 should have the partition key range ID header
        var range0Request = handler.SentRequests[2];
        await Assert.That(
            range0Request.Headers.GetValues("x-ms-documentdb-partitionkeyrangeid").First())
            .IsEqualTo("0");

        var range1Request = handler.SentRequests[3];
        await Assert.That(
            range1Request.Headers.GetValues("x-ms-documentdb-partitionkeyrangeid").First())
            .IsEqualTo("1");
    }

    [Test]
    public async Task Should_DrainContinuationPages_When_FanOutQueryHasMultiplePages()
    {
        var (client, handler) = CreateClient();

        // Request 1: cross-partition query returns 400 with fan-out marker
        // (uses non-DISTINCT query to exercise the runtime probe path)
        handler.EnqueueResponse(
            HttpStatusCode.BadRequest,
            """{"code":"BadRequest","message":"partitionedQueryExecutionInfoVersion"}""");

        // Request 2: GET pkranges returns one partition key range
        handler.EnqueueResponse(
            HttpStatusCode.OK,
            """{"PartitionKeyRanges":[{"id":"0"}],"_count":1}""");

        // Request 3: range 0 page 1 with continuation
        handler.EnqueueResponse(
            HttpStatusCode.OK,
            """{"Documents":[{"id":"d1","name":"Page1"}],"_count":1}""",
            [new("x-ms-continuation", "page2-token")]);

        // Request 4: range 0 page 2 (no continuation)
        handler.EnqueueResponse(
            HttpStatusCode.OK,
            """{"Documents":[{"id":"d2","name":"Page2"}],"_count":1}""");

        var results = await client.QueryAsync(
            "Users",
            "SELECT * FROM c WHERE ST_DISTANCE(c.location, @point) < 1000",
            null,
            null,
            TestSerializerContext.Default.TestDocument,
            CancellationToken.None);

        // Should have combined results from both pages
        await Assert.That(results).HasCount().EqualTo(2);
        await Assert.That(results[0].Name).IsEqualTo("Page1");
        await Assert.That(results[1].Name).IsEqualTo("Page2");

        // Should have sent 4 requests total
        await Assert.That(handler.SentRequests).HasCount().EqualTo(4);

        // Last request should have the continuation header
        var lastRequest = handler.SentRequests[3];
        await Assert.That(lastRequest.Headers.GetValues("x-ms-continuation").First())
            .IsEqualTo("page2-token");
    }

    [Test]
    public async Task Should_ThrowNormally_When_CrossPartitionBadRequestWithoutFanOutMarker()
    {
        var (client, handler) = CreateClient();

        // 400 without the fan-out marker
        handler.EnqueueResponse(
            HttpStatusCode.BadRequest,
            """{"code":"BadRequest","message":"syntax error"}""");

        var exception = await Assert.ThrowsAsync<HttpRequestException>(async () =>
            await client.QueryAsync(
                "Users",
                "SELECT * FROM c",
                null,
                null,
                TestSerializerContext.Default.TestDocument,
                CancellationToken.None));

        await Assert.That(exception.Message).Contains("syntax error");
    }

    [Test]
    public async Task Should_SkipProbeAndFanOutDirectly_When_DistinctCrossPartitionQuery()
    {
        var (client, handler) = CreateClient();

        // No 400 probe — first request is GET pkranges
        handler.EnqueueResponse(
            HttpStatusCode.OK,
            """{"PartitionKeyRanges":[{"id":"0"},{"id":"1"}],"_count":2}""");

        // Per-range queries return results directly
        handler.EnqueueResponse(
            HttpStatusCode.OK,
            """{"Documents":[{"id":"d1","name":"Alpha"}],"_count":1}""");
        handler.EnqueueResponse(
            HttpStatusCode.OK,
            """{"Documents":[{"id":"d2","name":"Beta"}],"_count":1}""");

        var results = await client.QueryAsync(
            "Users",
            "SELECT DISTINCT VALUE c.name FROM c",
            null,
            null,
            TestSerializerContext.Default.TestDocument,
            CancellationToken.None);

        // Results from both partitions
        await Assert.That(results).HasCount().EqualTo(2);

        // Only 3 requests: pkranges + 2 partition queries (no initial probe)
        await Assert.That(handler.SentRequests).HasCount().EqualTo(3);

        // First request is GET pkranges, NOT a POST query probe
        var firstRequest = handler.SentRequests[0];
        await Assert.That(firstRequest.Method).IsEqualTo(HttpMethod.Get);
        await Assert.That(firstRequest.RequestUri!.AbsolutePath)
            .IsEqualTo("/dbs/test-db/colls/Users/pkranges");
    }

    [Test]
    public async Task Should_SkipProbeAndFanOutDirectly_When_GroupByCrossPartitionQuery()
    {
        var (client, handler) = CreateClient();

        // No 400 probe — first request is GET pkranges
        handler.EnqueueResponse(
            HttpStatusCode.OK,
            """{"PartitionKeyRanges":[{"id":"0"}],"_count":1}""");

        // Single partition query
        handler.EnqueueResponse(
            HttpStatusCode.OK,
            """{"Documents":[{"id":"d1","name":"GroupResult"}],"_count":1}""");

        var results = await client.QueryAsync(
            "Users",
            "SELECT c.category, COUNT(1) AS total FROM c GROUP BY c.category",
            null,
            null,
            TestSerializerContext.Default.TestDocument,
            CancellationToken.None);

        await Assert.That(results).HasCount().EqualTo(1);

        // Only 2 requests: pkranges + 1 partition query (no initial probe)
        await Assert.That(handler.SentRequests).HasCount().EqualTo(2);
        await Assert.That(handler.SentRequests[0].Method).IsEqualTo(HttpMethod.Get);
    }

    [Test]
    public async Task Should_StillProbeNormally_When_SimpleCrossPartitionQuery()
    {
        var (client, handler) = CreateClient();

        // Simple cross-partition SELECT * should NOT skip to fan-out
        handler.EnqueueResponse(HttpStatusCode.OK, """{"Documents":[{"id":"d1","name":"Test"}],"_count":1}""");

        var results = await client.QueryAsync(
            "Users",
            "SELECT * FROM c WHERE c.active = true",
            null,
            null,
            TestSerializerContext.Default.TestDocument,
            CancellationToken.None);

        await Assert.That(results).HasCount().EqualTo(1);

        // Normal path: single POST query, no fan-out
        await Assert.That(handler.SentRequests).HasCount().EqualTo(1);
        await Assert.That(handler.SentRequests[0].Method).IsEqualTo(HttpMethod.Post);
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
