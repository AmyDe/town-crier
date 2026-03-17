using TownCrier.Application.Polling;

namespace TownCrier.Infrastructure.Polling;

public sealed class LogPollingHealthAlerter : IPollingHealthAlerter
{
    public Task AlertStalenessAsync(DateTimeOffset lastSuccessfulPoll, TimeSpan staleness, CancellationToken ct)
    {
        return Task.CompletedTask;
    }

    public Task AlertConsecutiveFailuresAsync(int failureCount, CancellationToken ct)
    {
        return Task.CompletedTask;
    }
}
