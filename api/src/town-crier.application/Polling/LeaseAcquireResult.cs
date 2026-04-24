namespace TownCrier.Application.Polling;

/// <summary>
/// Outcome of a lease acquire attempt.
/// </summary>
public sealed record LeaseAcquireResult
{
    public bool Acquired => this.Handle is not null;

    public LeaseHandle? Handle { get; init; }

    public bool Held { get; init; }

    public Exception? TransientError { get; init; }

    public static LeaseAcquireResult FromAcquired(LeaseHandle handle) => new() { Handle = handle };

    public static LeaseAcquireResult FromHeld() => new() { Held = true };

    public static LeaseAcquireResult FromTransient(Exception ex) => new() { TransientError = ex };
}
