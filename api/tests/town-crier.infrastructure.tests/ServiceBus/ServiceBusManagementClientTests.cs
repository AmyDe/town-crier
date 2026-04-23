using System.Diagnostics.CodeAnalysis;
using System.Net;
using TownCrier.Infrastructure.ServiceBus;
using TownCrier.Infrastructure.Tests.Cosmos;

namespace TownCrier.Infrastructure.Tests.ServiceBus;

[SuppressMessage("Reliability", "CA2000:Dispose objects before losing scope", Justification = "HttpClient lifetime managed by test")]
[SuppressMessage("Minor Code Smell", "S1075:URIs should not be hardcoded", Justification = "Test base address")]
public sealed class ServiceBusManagementClientTests
{
    private const string BaseUrl = "https://management.azure.com";
    private const string SubscriptionId = "ae5e40cd-96ef-48d8-950a-2e22cf8f991a";
    private const string ResourceGroup = "rg-town-crier-test";
    private const string Namespace = "sb-town-crier-test";
    private const string QueueName = "poll";

    [Test]
    public async Task Should_GetArmQueueResource_When_ReadingQueueDepth()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(
            HttpStatusCode.OK,
            """{"properties":{"countDetails":{"activeMessageCount":3,"scheduledMessageCount":7}}}""");

        var depth = await client.GetQueueDepthAsync(QueueName, CancellationToken.None);

        var request = handler.SentRequests[0];
        await Assert.That(request.Method).IsEqualTo(HttpMethod.Get);
        await Assert.That(request.RequestUri!.AbsolutePath).IsEqualTo(
            $"/subscriptions/{SubscriptionId}/resourceGroups/{ResourceGroup}"
            + $"/providers/Microsoft.ServiceBus/namespaces/{Namespace}/queues/{QueueName}");
        await Assert.That(request.RequestUri!.Query).Contains("api-version=2021-11-01");
        await Assert.That(depth.ActiveMessageCount).IsEqualTo(3L);
        await Assert.That(depth.ScheduledMessageCount).IsEqualTo(7L);
    }

    [Test]
    public async Task Should_AcceptFqdnNamespace_When_ReadingQueueDepth()
    {
        // Pulumi sets ServiceBus:Namespace to the FQDN; the management-plane URL
        // only wants the bare namespace name. The client must strip the suffix.
        var handler = new StubHttpHandler();
        var httpClient = new HttpClient(handler) { BaseAddress = new Uri(BaseUrl) };
        var credential = new StubTokenCredential("fake-arm-token");
        var authProvider = new AzureAdTokenProvider(
            credential,
            ["https://management.azure.com/.default"]);
        var options = new ServiceBusManagementOptions
        {
            SubscriptionId = SubscriptionId,
            ResourceGroup = ResourceGroup,
            Namespace = $"{Namespace}.servicebus.windows.net",
        };
        var client = new ServiceBusManagementClient(httpClient, authProvider, options);
        handler.EnqueueResponse(
            HttpStatusCode.OK,
            """{"properties":{"countDetails":{"activeMessageCount":0,"scheduledMessageCount":1}}}""");

        await client.GetQueueDepthAsync(QueueName, CancellationToken.None);

        var request = handler.SentRequests[0];
        await Assert.That(request.RequestUri!.AbsolutePath).Contains($"/namespaces/{Namespace}/queues/");
        await Assert.That(request.RequestUri!.AbsolutePath).DoesNotContain("servicebus.windows.net");
    }

    [Test]
    public async Task Should_SetBearerAuthorization_When_ReadingQueueDepth()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(
            HttpStatusCode.OK,
            """{"properties":{"countDetails":{"activeMessageCount":0,"scheduledMessageCount":0}}}""");

        await client.GetQueueDepthAsync(QueueName, CancellationToken.None);

        var request = handler.SentRequests[0];
        var auth = request.Headers.GetValues("Authorization").First();
        await Assert.That(auth).StartsWith("Bearer ");
    }

    [Test]
    public async Task Should_DefaultToZero_When_CountDetailsMissing()
    {
        // ARM may omit fields for freshly-created queues. Treat missing as zero
        // rather than throwing — bootstrap semantics are "if empty then seed".
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.OK, """{"properties":{}}""");

        var depth = await client.GetQueueDepthAsync(QueueName, CancellationToken.None);

        await Assert.That(depth.ActiveMessageCount).IsEqualTo(0L);
        await Assert.That(depth.ScheduledMessageCount).IsEqualTo(0L);
    }

    [Test]
    public async Task Should_ThrowHttpRequestException_When_GetQueueDepthFails()
    {
        var (client, handler) = CreateClient();
        handler.EnqueueResponse(HttpStatusCode.Unauthorized);

        var exception = await Assert.ThrowsAsync<HttpRequestException>(async () =>
            await client.GetQueueDepthAsync(QueueName, CancellationToken.None));

        await Assert.That(exception).IsNotNull();
    }

    private static (IServiceBusManagementClient Client, StubHttpHandler Handler) CreateClient()
    {
        var handler = new StubHttpHandler();
        var httpClient = new HttpClient(handler) { BaseAddress = new Uri(BaseUrl) };
        var credential = new StubTokenCredential("fake-arm-token");
        var authProvider = new AzureAdTokenProvider(
            credential,
            ["https://management.azure.com/.default"]);
        var options = new ServiceBusManagementOptions
        {
            SubscriptionId = SubscriptionId,
            ResourceGroup = ResourceGroup,
            Namespace = Namespace,
        };
        var client = new ServiceBusManagementClient(httpClient, authProvider, options);
        return (client, handler);
    }
}
