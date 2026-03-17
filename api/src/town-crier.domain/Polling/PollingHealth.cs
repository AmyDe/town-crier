namespace TownCrier.Domain.Polling;

public sealed class PollingHealth
{
    public DateTimeOffset? LastSuccessfulPollTime { get; private set; }

    public int ConsecutiveFailureCount { get; private set; }

    public void RecordSuccess(DateTimeOffset pollTime)
    {
        this.LastSuccessfulPollTime = pollTime;
        this.ConsecutiveFailureCount = 0;
    }

    public void RecordFailure()
    {
        this.ConsecutiveFailureCount++;
    }

    public bool IsStale(DateTimeOffset now, TimeSpan stalenessThreshold)
    {
        if (this.LastSuccessfulPollTime is null)
        {
            return false;
        }

        return now - this.LastSuccessfulPollTime.Value > stalenessThreshold;
    }

    public bool HasExceededFailureThreshold(int maxConsecutiveFailures)
    {
        return this.ConsecutiveFailureCount >= maxConsecutiveFailures;
    }
}
