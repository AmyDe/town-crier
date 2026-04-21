using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

internal sealed class FakePollTriggerQueue : IPollTriggerQueue
{
    private readonly Queue<IPollTriggerMessage> receivable = new();

    public List<DateTimeOffset> ScheduledEnqueueTimes { get; } = new();

    public int CompletedCount { get; private set; }

    public int AbandonedCount { get; private set; }

    public List<string> ScheduleSequence { get; } = new();

    public void EnqueueReceivable(IPollTriggerMessage message)
    {
        this.receivable.Enqueue(message);
    }

    public Task<IPollTriggerMessage?> ReceiveAsync(CancellationToken ct)
    {
        if (this.receivable.Count == 0)
        {
            return Task.FromResult<IPollTriggerMessage?>(null);
        }

        return Task.FromResult<IPollTriggerMessage?>(this.receivable.Dequeue());
    }

    public Task PublishAtAsync(DateTimeOffset scheduledEnqueueTime, CancellationToken ct)
    {
        this.ScheduleSequence.Add("publish");
        this.ScheduledEnqueueTimes.Add(scheduledEnqueueTime);
        return Task.CompletedTask;
    }

    public Task CompleteAsync(IPollTriggerMessage message, CancellationToken ct)
    {
        this.ScheduleSequence.Add("complete");
        this.CompletedCount++;
        return Task.CompletedTask;
    }

    public Task AbandonAsync(IPollTriggerMessage message, CancellationToken ct)
    {
        this.ScheduleSequence.Add("abandon");
        this.AbandonedCount++;
        return Task.CompletedTask;
    }
}
