using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

internal sealed class SpyPollingHealthAlerter : IPollingHealthAlerter
{
    public List<(DateTimeOffset LastSuccessfulPoll, TimeSpan Staleness)> StalenessAlerts { get; } = [];

    public List<int> ConsecutiveFailureAlerts { get; } = [];

    public Task AlertStalenessAsync(DateTimeOffset lastSuccessfulPoll, TimeSpan staleness, CancellationToken ct)
    {
        this.StalenessAlerts.Add((lastSuccessfulPoll, staleness));
        return Task.CompletedTask;
    }

    public Task AlertConsecutiveFailuresAsync(int failureCount, CancellationToken ct)
    {
        this.ConsecutiveFailureAlerts.Add(failureCount);
        return Task.CompletedTask;
    }
}
