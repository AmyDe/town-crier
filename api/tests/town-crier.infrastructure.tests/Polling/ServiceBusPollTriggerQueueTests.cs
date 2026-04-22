using System.Diagnostics.CodeAnalysis;
using TownCrier.Application.Polling;
using TownCrier.Infrastructure.Polling;
using TownCrier.Infrastructure.ServiceBus;

namespace TownCrier.Infrastructure.Tests.Polling;

/// <summary>
/// Adapter-level tests for <see cref="ServiceBusPollTriggerQueue"/> under
/// ADR 0024 amendment (receive-and-delete). Destructive consume on receive —
/// no lock, no Complete, no Abandon.
/// </summary>
[SuppressMessage("Minor Code Smell", "S1075:URIs should not be hardcoded", Justification = "Test fixture URIs")]
public sealed class ServiceBusPollTriggerQueueTests
{
    private const string QueueName = "poll";

    [Test]
    public async Task Should_PublishToConfiguredQueue_When_PublishAt()
    {
        var rest = new FakeServiceBusRestClient();
        var queue = new ServiceBusPollTriggerQueue(
            rest,
            new ServiceBusRestOptions { Namespace = "sb-test", QueueName = QueueName });

        var scheduledAt = new DateTimeOffset(2026, 4, 21, 12, 0, 0, TimeSpan.Zero);
        await queue.PublishAtAsync(scheduledAt, CancellationToken.None);

        await Assert.That(rest.PublishCalls).HasCount().EqualTo(1);
        await Assert.That(rest.PublishCalls[0].QueueName).IsEqualTo(QueueName);
        await Assert.That(rest.PublishCalls[0].ScheduledEnqueueTimeUtc).IsEqualTo(scheduledAt);
    }

    [Test]
    public async Task Should_ReturnNull_When_ReceiveQueueIsEmpty()
    {
        var rest = new FakeServiceBusRestClient();
        var queue = new ServiceBusPollTriggerQueue(
            rest,
            new ServiceBusRestOptions { Namespace = "sb-test", QueueName = QueueName });

        var message = await queue.ReceiveAsync(CancellationToken.None);

        await Assert.That(message).IsNull();
    }

    [Test]
    public async Task Should_ReturnMessage_When_ReceiveReturnsBody()
    {
        var rest = new FakeServiceBusRestClient();
        rest.EnqueueReceive(new ReceivedServiceBusMessage
        {
            Body = [1, 2, 3],
        });
        var queue = new ServiceBusPollTriggerQueue(
            rest,
            new ServiceBusRestOptions { Namespace = "sb-test", QueueName = QueueName });

        var message = await queue.ReceiveAsync(CancellationToken.None);

        await Assert.That(message).IsNotNull();
    }

    [Test]
    public async Task Should_PublishSerialisedPayload_When_PublishAt()
    {
        var rest = new FakeServiceBusRestClient();
        var queue = new ServiceBusPollTriggerQueue(
            rest,
            new ServiceBusRestOptions { Namespace = "sb-test", QueueName = QueueName });

        await queue.PublishAtAsync(
            new DateTimeOffset(2026, 4, 21, 12, 0, 0, TimeSpan.Zero),
            CancellationToken.None);

        // Payload is JSON — at minimum, non-empty object-shaped content.
        await Assert.That(rest.PublishCalls[0].SerialisedBody).IsNotNull();
        await Assert.That(rest.PublishCalls[0].SerialisedBody).StartsWith("{");
    }
}
