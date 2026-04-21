using System.Diagnostics.CodeAnalysis;
using System.Net;
using TownCrier.Infrastructure.ServiceBus;
using TownCrier.Infrastructure.Tests.Cosmos;

namespace TownCrier.Infrastructure.Tests.ServiceBus;

[SuppressMessage("Reliability", "CA2000:Dispose objects before losing scope", Justification = "HttpClient lifetime managed by test")]
public sealed class ServiceBusRestClientTests
{
    private const string Namespace = "sb-town-crier-test";
    private const string BaseUrl = "https://sb-town-crier-test.servicebus.windows.net";
    private const string QueueName = "poll";

    [Test]
    public async Task Should_PostToMessagesEndpoint_When_PublishingPayload()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.Created);

        await client.PublishAsync(
            QueueName,
            new TestDocument { Id = "d1", Name = "Hello" },
            scheduledEnqueueTimeUtc: null,
            TestSerializerContext.Default.TestDocument,
            CancellationToken.None);

        var request = handler.SentRequests[0];
        await Assert.That(request.Method).IsEqualTo(HttpMethod.Post);
        await Assert.That(request.RequestUri!.AbsolutePath).IsEqualTo("/poll/messages");
        await Assert.That(request.RequestUri!.Query).Contains("api-version=2015-01");
    }

    [Test]
    public async Task Should_SetBearerAuthorization_When_Publishing()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.Created);

        await client.PublishAsync(
            QueueName,
            new TestDocument { Id = "d1", Name = "Hello" },
            scheduledEnqueueTimeUtc: null,
            TestSerializerContext.Default.TestDocument,
            CancellationToken.None);

        var request = handler.SentRequests[0];
        var auth = request.Headers.GetValues("Authorization").First();
        await Assert.That(auth).StartsWith("Bearer ");
    }

    [Test]
    public async Task Should_SetBrokerPropertiesHeader_When_ScheduledEnqueueTimeProvided()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.Created);

        var scheduledAt = new DateTimeOffset(2026, 5, 1, 12, 0, 0, TimeSpan.Zero);

        await client.PublishAsync(
            QueueName,
            new TestDocument { Id = "d1", Name = "Hello" },
            scheduledAt,
            TestSerializerContext.Default.TestDocument,
            CancellationToken.None);

        var request = handler.SentRequests[0];
        var brokerProps = request.Headers.GetValues("BrokerProperties").First();
        await Assert.That(brokerProps).Contains("ScheduledEnqueueTimeUtc");
        await Assert.That(brokerProps).Contains("2026");
    }

    [Test]
    public async Task Should_NotSetBrokerPropertiesHeader_When_NoScheduledEnqueueTime()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.Created);

        await client.PublishAsync(
            QueueName,
            new TestDocument { Id = "d1", Name = "Hello" },
            scheduledEnqueueTimeUtc: null,
            TestSerializerContext.Default.TestDocument,
            CancellationToken.None);

        var request = handler.SentRequests[0];
        await Assert.That(request.Headers.Contains("BrokerProperties")).IsFalse();
    }

    [Test]
    public async Task Should_ReturnMessage_When_ReceiveOneSucceeds()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(
            HttpStatusCode.Created,
            """{"id":"m1","name":"Queued"}""",
            [new("Location", $"{BaseUrl}/poll/messages/abc-123/lock-xyz")]);

        var message = await client.ReceiveOneAsync(
            QueueName,
            TimeSpan.FromSeconds(30),
            CancellationToken.None);

        await Assert.That(message).IsNotNull();
        await Assert.That(message!.LockUrl.ToString())
            .IsEqualTo($"{BaseUrl}/poll/messages/abc-123/lock-xyz");
    }

    [Test]
    public async Task Should_PostToPeekLockEndpoint_When_ReceivingOne()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(
            HttpStatusCode.Created,
            """{"id":"m1","name":"Queued"}""",
            [new("Location", $"{BaseUrl}/poll/messages/abc/lock")]);

        await client.ReceiveOneAsync(
            QueueName,
            TimeSpan.FromSeconds(45),
            CancellationToken.None);

        var request = handler.SentRequests[0];
        await Assert.That(request.Method).IsEqualTo(HttpMethod.Post);
        await Assert.That(request.RequestUri!.AbsolutePath).IsEqualTo("/poll/messages/head");
        await Assert.That(request.RequestUri!.Query).Contains("timeout=45");
        await Assert.That(request.RequestUri!.Query).Contains("api-version=2015-01");
    }

    [Test]
    public async Task Should_ReturnNull_When_ReceiveOneHasNoMessages()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.NoContent);

        var message = await client.ReceiveOneAsync(
            QueueName,
            TimeSpan.FromSeconds(5),
            CancellationToken.None);

        await Assert.That(message).IsNull();
    }

    [Test]
    public async Task Should_DeleteLockUrl_When_Completing()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.OK);

        var lockUrl = new Uri($"{BaseUrl}/poll/messages/msg-1/lock-abc");

        await client.CompleteAsync(lockUrl, CancellationToken.None);

        var request = handler.SentRequests[0];
        await Assert.That(request.Method).IsEqualTo(HttpMethod.Delete);
        await Assert.That(request.RequestUri!.ToString()).IsEqualTo(lockUrl.ToString());
        await Assert.That(request.Headers.Contains("Authorization")).IsTrue();
    }

    [Test]
    public async Task Should_ThrowHttpRequestException_When_PublishFails()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.BadRequest);

        var exception = await Assert.ThrowsAsync<HttpRequestException>(async () =>
            await client.PublishAsync(
                QueueName,
                new TestDocument { Id = "d1", Name = "x" },
                scheduledEnqueueTimeUtc: null,
                TestSerializerContext.Default.TestDocument,
                CancellationToken.None));

        await Assert.That(exception).IsNotNull();
    }

    private static (IServiceBusRestClient Client, StubHttpHandler Handler) CreateClient()
    {
        var handler = new StubHttpHandler();
        var httpClient = new HttpClient(handler) { BaseAddress = new Uri(BaseUrl) };
        var credential = new StubTokenCredential("fake-token");
        var authProvider = new ServiceBusAuthProvider(credential);
        var options = new ServiceBusRestOptions
        {
            Namespace = Namespace,
            QueueName = QueueName,
        };
        var client = new ServiceBusRestClient(httpClient, authProvider, options);
        return (client, handler);
    }
}
