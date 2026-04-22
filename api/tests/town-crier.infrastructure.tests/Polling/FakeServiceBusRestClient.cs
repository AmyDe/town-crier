using System.Text.Json;
using System.Text.Json.Serialization.Metadata;
using TownCrier.Infrastructure.ServiceBus;

namespace TownCrier.Infrastructure.Tests.Polling;

internal sealed class FakeServiceBusRestClient : IServiceBusRestClient
{
    private readonly Queue<ReceivedServiceBusMessage?> receiveResults = new();
    private readonly Queue<Exception> receiveThrows = new();
    private readonly Queue<ServiceBusQueueCountDetails> depthResults = new();

    public List<PublishCall> PublishCalls { get; } = [];

    /// <summary>
    /// Gets the ordered log of every mutating call ("publish") plus reads
    /// ("receive", "depth"). Used by integration tests to assert the
    /// receive-then-handler-then-publish ordering contract.
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

    public void EnqueueDepth(long active, long scheduled)
    {
        this.depthResults.Enqueue(new ServiceBusQueueCountDetails(active, scheduled));
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
        this.CallSequence.Add("receive");

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

    public Task<ServiceBusQueueCountDetails> GetQueueDepthAsync(string queueName, CancellationToken ct)
    {
        this.CallSequence.Add("depth");
        if (this.depthResults.Count == 0)
        {
            return Task.FromResult(new ServiceBusQueueCountDetails(0, 0));
        }

        return Task.FromResult(this.depthResults.Dequeue());
    }

    public sealed record PublishCall(
        string QueueName,
        DateTimeOffset? ScheduledEnqueueTimeUtc,
        string SerialisedBody);
}
