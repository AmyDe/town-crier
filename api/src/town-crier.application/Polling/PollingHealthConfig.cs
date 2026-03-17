namespace TownCrier.Application.Polling;

public sealed record PollingHealthConfig(
    TimeSpan StalenessThreshold,
    int MaxConsecutiveFailures);
