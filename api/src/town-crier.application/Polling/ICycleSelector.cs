namespace TownCrier.Application.Polling;

public interface ICycleSelector
{
    CycleType GetCurrent();
}
