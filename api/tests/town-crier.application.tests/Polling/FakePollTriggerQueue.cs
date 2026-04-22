using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

internal sealed class FakePollTriggerQueue : IPollTriggerQueue
{
    private readonly Queue<IPollTriggerMessage> receivable = new();

    public List<DateTimeOffset> ScheduledEnqueueTimes { get; } = new();

    public int ReceiveCount { get; private set; }

    public List<string> CallSequence { get; } = new();

    public void EnqueueReceivable(IPollTriggerMessage message)
    {
        this.receivable.Enqueue(message);
    }

    public Task<IPollTriggerMessage?> ReceiveAsync(CancellationToken ct)
    {
        this.ReceiveCount++;
        this.CallSequence.Add("receive");
        if (this.receivable.Count == 0)
        {
            return Task.FromResult<IPollTriggerMessage?>(null);
        }

        return Task.FromResult<IPollTriggerMessage?>(this.receivable.Dequeue());
    }

    public Task PublishAtAsync(DateTimeOffset scheduledEnqueueTime, CancellationToken ct)
    {
        this.CallSequence.Add("publish");
        this.ScheduledEnqueueTimes.Add(scheduledEnqueueTime);
        return Task.CompletedTask;
    }
}
