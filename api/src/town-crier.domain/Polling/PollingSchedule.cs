namespace TownCrier.Domain.Polling;

public sealed class PollingSchedule
{
    private readonly Dictionary<int, PollingPriority> priorities;

    private PollingSchedule(Dictionary<int, PollingPriority> priorities)
    {
        this.priorities = priorities;
    }

    public IReadOnlyCollection<int> AuthorityIds => this.priorities.Keys;

    public static PollingSchedule Calculate(
        Dictionary<int, int> zoneCounts,
        PollingScheduleConfig config)
    {
        ArgumentNullException.ThrowIfNull(zoneCounts);
        ArgumentNullException.ThrowIfNull(config);

        var priorities = new Dictionary<int, PollingPriority>();

        foreach (var (authorityId, count) in zoneCounts)
        {
            var priority = ClassifyPriority(count, config);
            priorities[authorityId] = priority;
        }

        return new PollingSchedule(priorities);
    }

    public PollingPriority GetPriority(int authorityId)
    {
        return this.priorities.TryGetValue(authorityId, out var priority)
            ? priority
            : PollingPriority.Low;
    }

    public bool ShouldPollInCycle(int authorityId, int cycleNumber)
    {
        var priority = this.GetPriority(authorityId);

        return priority switch
        {
            PollingPriority.High => true,
            PollingPriority.Normal => cycleNumber % 2 == 0,
            PollingPriority.Low => cycleNumber % 4 == 0,
            _ => false,
        };
    }

    private static PollingPriority ClassifyPriority(int zoneCount, PollingScheduleConfig config)
    {
        if (zoneCount >= config.HighThreshold)
        {
            return PollingPriority.High;
        }

        return zoneCount >= config.LowThreshold
            ? PollingPriority.Normal
            : PollingPriority.Low;
    }
}
