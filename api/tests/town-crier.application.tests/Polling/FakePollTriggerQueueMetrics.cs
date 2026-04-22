using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

internal sealed class FakePollTriggerQueueMetrics : IPollTriggerQueueMetrics
{
    private readonly Queue<PollTriggerQueueDepth> depths = new();
    private readonly Queue<Exception> throws = new();

    public int GetDepthCallCount { get; private set; }

    public void Enqueue(long active, long scheduled)
    {
        this.depths.Enqueue(new PollTriggerQueueDepth(active, scheduled));
    }

    public void EnqueueThrow(Exception exception)
    {
        this.throws.Enqueue(exception);
    }

    public Task<PollTriggerQueueDepth> GetDepthAsync(CancellationToken ct)
    {
        this.GetDepthCallCount++;

        if (this.throws.Count > 0)
        {
            throw this.throws.Dequeue();
        }

        if (this.depths.Count == 0)
        {
            return Task.FromResult(new PollTriggerQueueDepth(0, 0));
        }

        return Task.FromResult(this.depths.Dequeue());
    }
}
