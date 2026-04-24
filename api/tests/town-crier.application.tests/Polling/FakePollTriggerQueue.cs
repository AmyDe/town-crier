using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

internal sealed class FakePollTriggerQueue : IPollTriggerQueue
{
    private readonly Queue<IPollTriggerMessage> receivable = new();
    private int publishCallCount;

    public List<DateTimeOffset> ScheduledEnqueueTimes { get; } = new();

    public int ReceiveCount { get; private set; }

    /// <summary>
    /// Gets the number of times <see cref="ReceiveAsync"/> has been called.
    /// Alias of <see cref="ReceiveCount"/> — used by orchestrator tests.
    /// </summary>
    public int ReceiveCalls => this.ReceiveCount;

    /// <summary>
    /// Gets the number of times <see cref="PublishAtAsync"/> completed successfully
    /// (i.e. without throwing).
    /// </summary>
    public int PublishCalls => this.publishCallCount;

    public List<string> CallSequence { get; } = new();

    /// <summary>
    /// Gets or sets an exception to throw from the next <see cref="PublishAtAsync"/> call.
    /// Cleared after first use.
    /// </summary>
    public Exception? ThrowOnPublish { get; set; }

    public void EnqueueReceivable(IPollTriggerMessage message)
    {
        this.receivable.Enqueue(message);
    }

    /// <summary>
    /// Enqueues a synthetic trigger message with a generated ID so callers
    /// that don't care about the message identity can use a shorter form.
    /// </summary>
    public void EnqueueReceivable()
    {
        this.receivable.Enqueue(new FakePollTriggerMessage($"M-{Guid.NewGuid():N}"));
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

        if (this.ThrowOnPublish is { } ex)
        {
            this.ThrowOnPublish = null;
            throw ex;
        }

        this.ScheduledEnqueueTimes.Add(scheduledEnqueueTime);
        this.publishCallCount++;
        return Task.CompletedTask;
    }
}
