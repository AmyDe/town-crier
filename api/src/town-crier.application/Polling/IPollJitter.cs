namespace TownCrier.Application.Polling;

/// <summary>
/// Returns a jitter offset in the range [-bound, +bound]. Port so tests can inject
/// deterministic offsets.
/// </summary>
public interface IPollJitter
{
    TimeSpan NextOffset(TimeSpan bound);
}
