namespace TownCrier.Application.Polling;

public interface IPollStateStore
{
    Task<DateTimeOffset?> GetLastPollTimeAsync(CancellationToken ct);

    Task SaveLastPollTimeAsync(DateTimeOffset pollTime, CancellationToken ct);
}
