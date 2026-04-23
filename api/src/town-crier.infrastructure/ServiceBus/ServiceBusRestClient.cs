using System.Globalization;
using System.Net;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization.Metadata;

namespace TownCrier.Infrastructure.ServiceBus;

internal sealed class ServiceBusRestClient : IServiceBusRestClient
{
    /// <summary>
    /// API version used for runtime queue operations (publish, receive-and-delete).
    /// </summary>
    private const string RuntimeApiVersion = "2015-01";

    private readonly HttpClient httpClient;
    private readonly AzureAdTokenProvider authProvider;

    public ServiceBusRestClient(
        HttpClient httpClient,
        AzureAdTokenProvider authProvider,
        ServiceBusRestOptions options)
    {
        ArgumentNullException.ThrowIfNull(httpClient);
        ArgumentNullException.ThrowIfNull(authProvider);
        ArgumentNullException.ThrowIfNull(options);

        this.httpClient = httpClient;
        this.authProvider = authProvider;
    }

    public async Task PublishAsync<T>(
        string queueName,
        T payload,
        DateTimeOffset? scheduledEnqueueTimeUtc,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrEmpty(queueName);
        ArgumentNullException.ThrowIfNull(typeInfo);

        using var request = new HttpRequestMessage(
            HttpMethod.Post,
            $"/{queueName}/messages?api-version={RuntimeApiVersion}");

        await this.AddAuthorizationHeaderAsync(request, ct).ConfigureAwait(false);

        if (scheduledEnqueueTimeUtc.HasValue)
        {
            var brokerProps = new BrokerProperties
            {
                ScheduledEnqueueTimeUtc = scheduledEnqueueTimeUtc.Value
                    .UtcDateTime
                    .ToString("R", CultureInfo.InvariantCulture),
            };
            var brokerJson = JsonSerializer.Serialize(
                brokerProps,
                ServiceBusJsonSerializerContext.Default.BrokerProperties);
            request.Headers.TryAddWithoutValidation("BrokerProperties", brokerJson);
        }

        request.Content = new StringContent(
            JsonSerializer.Serialize(payload, typeInfo),
            Encoding.UTF8,
            "application/json");

        using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);
        await ThrowOnFailureAsync(response, "Publish", ct).ConfigureAwait(false);
    }

    public async Task<ReceivedServiceBusMessage?> ReceiveOneAsync(
        string queueName,
        TimeSpan timeout,
        CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrEmpty(queueName);

        var seconds = (int)Math.Max(1, timeout.TotalSeconds);

        // Receive-and-delete mode: DELETE /{queue}/messages/head destructively
        // consumes one message in a single round-trip. No lock, no Complete,
        // no Abandon. See ADR 0024 amendment (2026-04-22).
        using var request = new HttpRequestMessage(
            HttpMethod.Delete,
            $"/{queueName}/messages/head?timeout={seconds.ToString(CultureInfo.InvariantCulture)}&api-version={RuntimeApiVersion}");

        await this.AddAuthorizationHeaderAsync(request, ct).ConfigureAwait(false);

        using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);

        // Empty queue: Service Bus returns 204 No Content, occasionally 404.
        if (response.StatusCode == HttpStatusCode.NoContent
            || response.StatusCode == HttpStatusCode.NotFound)
        {
            return null;
        }

        await ThrowOnFailureAsync(response, "ReceiveOne", ct).ConfigureAwait(false);

        var body = await response.Content.ReadAsByteArrayAsync(ct).ConfigureAwait(false);

        return new ReceivedServiceBusMessage
        {
            Body = body,
        };
    }

    private static async Task ThrowOnFailureAsync(
        HttpResponseMessage response,
        string operation,
        CancellationToken ct)
    {
        if (response.IsSuccessStatusCode)
        {
            return;
        }

        var body = await response.Content.ReadAsStringAsync(ct).ConfigureAwait(false);
        throw new HttpRequestException(
            $"Service Bus {operation} failed ({(int)response.StatusCode}): {body}");
    }

    private async Task AddAuthorizationHeaderAsync(HttpRequestMessage request, CancellationToken ct)
    {
        var auth = await this.authProvider.GetAuthorizationHeaderAsync(ct).ConfigureAwait(false);
        request.Headers.TryAddWithoutValidation("Authorization", auth);
    }
}
