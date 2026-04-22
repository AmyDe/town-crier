namespace TownCrier.Application.Tests.Polling;

internal sealed class FakeTimeProvider : TimeProvider
{
    private DateTimeOffset utcNow;

    public FakeTimeProvider(DateTimeOffset utcNow)
    {
        this.utcNow = utcNow;
    }

    public override DateTimeOffset GetUtcNow() => this.utcNow;

    public void Advance(TimeSpan delta)
    {
        this.utcNow += delta;
    }
}
