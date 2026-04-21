using System.Diagnostics.CodeAnalysis;
using TownCrier.Application.Polling;
using TownCrier.Infrastructure.Polling;
using TownCrier.Infrastructure.ServiceBus;

namespace TownCrier.Infrastructure.Tests.Polling;

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
    public async Task Should_WrapMessage_When_ReceiveReturnsMessage()
    {
        var rest = new FakeServiceBusRestClient();
        var lockUrl = new Uri("https://sb-test.servicebus.windows.net/poll/messages/abc/lock-1");
        rest.EnqueueReceive(new ReceivedServiceBusMessage
        {
            Body = [1, 2, 3],
            LockUrl = lockUrl,
        });
        var queue = new ServiceBusPollTriggerQueue(
            rest,
            new ServiceBusRestOptions { Namespace = "sb-test", QueueName = QueueName });

        var message = await queue.ReceiveAsync(CancellationToken.None);

        await Assert.That(message).IsNotNull();
        await Assert.That(message!.Id).IsEqualTo(lockUrl.ToString());
    }

    [Test]
    public async Task Should_CompleteLockUrl_When_Completing()
    {
        var rest = new FakeServiceBusRestClient();
        var lockUrl = new Uri("https://sb-test.servicebus.windows.net/poll/messages/abc/lock-2");
        rest.EnqueueReceive(new ReceivedServiceBusMessage
        {
            Body = [],
            LockUrl = lockUrl,
        });
        var queue = new ServiceBusPollTriggerQueue(
            rest,
            new ServiceBusRestOptions { Namespace = "sb-test", QueueName = QueueName });

        var message = await queue.ReceiveAsync(CancellationToken.None);
        await queue.CompleteAsync(message!, CancellationToken.None);

        await Assert.That(rest.CompletedLockUrls).HasCount().EqualTo(1);
        await Assert.That(rest.CompletedLockUrls[0]).IsEqualTo(lockUrl);
    }

    [Test]
    public async Task Should_AbandonLockUrl_When_Abandoning()
    {
        var rest = new FakeServiceBusRestClient();
        var lockUrl = new Uri("https://sb-test.servicebus.windows.net/poll/messages/abc/lock-3");
        rest.EnqueueReceive(new ReceivedServiceBusMessage
        {
            Body = [],
            LockUrl = lockUrl,
        });
        var queue = new ServiceBusPollTriggerQueue(
            rest,
            new ServiceBusRestOptions { Namespace = "sb-test", QueueName = QueueName });

        var message = await queue.ReceiveAsync(CancellationToken.None);
        await queue.AbandonAsync(message!, CancellationToken.None);

        await Assert.That(rest.AbandonedLockUrls).HasCount().EqualTo(1);
        await Assert.That(rest.AbandonedLockUrls[0]).IsEqualTo(lockUrl);
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

    [Test]
    public async Task Should_Throw_When_CompletingForeignMessage()
    {
        var rest = new FakeServiceBusRestClient();
        var queue = new ServiceBusPollTriggerQueue(
            rest,
            new ServiceBusRestOptions { Namespace = "sb-test", QueueName = QueueName });

        var foreign = new ForeignPollTriggerMessage("foreign");

        await Assert.ThrowsAsync<ArgumentException>(
            async () => await queue.CompleteAsync(foreign, CancellationToken.None));
    }

    private sealed class ForeignPollTriggerMessage : IPollTriggerMessage
    {
        public ForeignPollTriggerMessage(string id) => this.Id = id;

        public string Id { get; }
    }
}
