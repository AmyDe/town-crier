using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

internal sealed class FakeCycleSelector : ICycleSelector
{
    public FakeCycleSelector(CycleType cycleType = CycleType.Watched)
    {
        this.Current = cycleType;
    }

    public CycleType Current { get; set; }

    public int GetCurrentCallCount { get; private set; }

    public CycleType GetCurrent()
    {
        this.GetCurrentCallCount++;
        return this.Current;
    }
}
