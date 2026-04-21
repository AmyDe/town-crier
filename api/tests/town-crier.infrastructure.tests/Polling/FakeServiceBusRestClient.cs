using System.Text.Json;
using System.Text.Json.Serialization.Metadata;
using TownCrier.Infrastructure.ServiceBus;

namespace TownCrier.Infrastructure.Tests.Polling;

internal sealed class FakeServiceBusRestClient : IServiceBusRestClient
{
    private readonly Queue<ReceivedServiceBusMessage?> receiveResults = new();
    private readonly Queue<Exception> receiveThrows = new();

    public List<PublishCall> PublishCalls { get; } = [];

    public List<Uri> CompletedLockUrls { get; } = [];

    public List<Uri> AbandonedLockUrls { get; } = [];

    /// <summary>
    /// Gets the ordered log of every mutating call ("publish", "complete", "abandon")
    /// used by integration tests to assert the publish-before-ack ordering contract.
    /// </summary>
    public List<string> CallSequence { get; } = [];

    public void EnqueueReceive(ReceivedServiceBusMessage? message)
    {
        this.receiveResults.Enqueue(message);
    }

    public void EnqueueReceiveThrow(Exception exception)
    {
        this.receiveThrows.Enqueue(exception);
    }

    public Task PublishAsync<T>(
        string queueName,
        T payload,
        DateTimeOffset? scheduledEnqueueTimeUtc,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        var serialised = JsonSerializer.Serialize(payload, typeInfo);
        this.PublishCalls.Add(new PublishCall(
            QueueName: queueName,
            ScheduledEnqueueTimeUtc: scheduledEnqueueTimeUtc,
            SerialisedBody: serialised));
        this.CallSequence.Add("publish");
        return Task.CompletedTask;
    }

    public Task<ReceivedServiceBusMessage?> ReceiveOneAsync(
        string queueName,
        TimeSpan timeout,
        CancellationToken ct)
    {
        if (this.receiveThrows.Count > 0)
        {
            throw this.receiveThrows.Dequeue();
        }

        if (this.receiveResults.Count == 0)
        {
            return Task.FromResult<ReceivedServiceBusMessage?>(null);
        }

        return Task.FromResult(this.receiveResults.Dequeue());
    }

    public Task CompleteAsync(Uri lockUrl, CancellationToken ct)
    {
        this.CompletedLockUrls.Add(lockUrl);
        this.CallSequence.Add("complete");
        return Task.CompletedTask;
    }

    public Task AbandonAsync(Uri lockUrl, CancellationToken ct)
    {
        this.AbandonedLockUrls.Add(lockUrl);
        this.CallSequence.Add("abandon");
        return Task.CompletedTask;
    }

    public sealed record PublishCall(
        string QueueName,
        DateTimeOffset? ScheduledEnqueueTimeUtc,
        string SerialisedBody);
}
