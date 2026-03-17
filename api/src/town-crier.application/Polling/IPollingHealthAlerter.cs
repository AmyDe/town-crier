namespace TownCrier.Application.Polling;

public interface IPollingHealthAlerter
{
    Task AlertStalenessAsync(DateTimeOffset lastSuccessfulPoll, TimeSpan staleness, CancellationToken ct);

    Task AlertConsecutiveFailuresAsync(int failureCount, CancellationToken ct);
}
