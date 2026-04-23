using System.Globalization;
using System.Text.Json;

namespace TownCrier.Infrastructure.ServiceBus;

internal sealed class ServiceBusManagementClient : IServiceBusManagementClient
{
    private const string ApiVersion = "2021-11-01";
    private const string DataPlaneSuffix = ".servicebus.windows.net";

    private readonly HttpClient httpClient;
    private readonly AzureAdTokenProvider authProvider;
    private readonly ServiceBusManagementOptions options;

    public ServiceBusManagementClient(
        HttpClient httpClient,
        AzureAdTokenProvider authProvider,
        ServiceBusManagementOptions options)
    {
        ArgumentNullException.ThrowIfNull(httpClient);
        ArgumentNullException.ThrowIfNull(authProvider);
        ArgumentNullException.ThrowIfNull(options);

        this.httpClient = httpClient;
        this.authProvider = authProvider;
        this.options = options;
    }

    public async Task<ServiceBusQueueCountDetails> GetQueueDepthAsync(string queueName, CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrEmpty(queueName);

        // Pulumi sets ServiceBus:Namespace to the FQDN so the data-plane client
        // can reach <ns>.servicebus.windows.net. ARM wants the bare resource
        // name, so strip the suffix here rather than threading a second env var.
        var namespaceName = this.options.Namespace.EndsWith(DataPlaneSuffix, StringComparison.OrdinalIgnoreCase)
            ? this.options.Namespace[..^DataPlaneSuffix.Length]
            : this.options.Namespace;

        var path = string.Create(
            CultureInfo.InvariantCulture,
            $"/subscriptions/{this.options.SubscriptionId}/resourceGroups/{this.options.ResourceGroup}/providers/Microsoft.ServiceBus/namespaces/{namespaceName}/queues/{queueName}?api-version={ApiVersion}");

        using var request = new HttpRequestMessage(HttpMethod.Get, path);
        var auth = await this.authProvider.GetAuthorizationHeaderAsync(ct).ConfigureAwait(false);
        request.Headers.TryAddWithoutValidation("Authorization", auth);

        using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);
        if (!response.IsSuccessStatusCode)
        {
            var body = await response.Content.ReadAsStringAsync(ct).ConfigureAwait(false);
            throw new HttpRequestException(
                $"Service Bus GetQueueDepth failed ({(int)response.StatusCode}): {body}");
        }

        var payload = await response.Content.ReadAsStringAsync(ct).ConfigureAwait(false);
        var parsed = JsonSerializer.Deserialize(
            payload,
            ServiceBusJsonSerializerContext.Default.QueueCountDetailsResponse);

        var counts = parsed?.Properties?.CountDetails;
        return new ServiceBusQueueCountDetails(
            ActiveMessageCount: counts?.ActiveMessageCount ?? 0,
            ScheduledMessageCount: counts?.ScheduledMessageCount ?? 0);
    }
}
