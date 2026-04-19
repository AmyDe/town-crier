namespace TownCrier.Application.Polling;

public sealed class MinuteBasedCycleSelector : ICycleSelector
{
    private readonly TimeProvider timeProvider;

    public MinuteBasedCycleSelector(TimeProvider timeProvider)
    {
        this.timeProvider = timeProvider;
    }

    public CycleType GetCurrent()
    {
        var minute = this.timeProvider.GetUtcNow().Minute;
        return (minute % 30) < 15 ? CycleType.Watched : CycleType.Seed;
    }
}
