namespace TownCrier.Application.Tests.Groups;

internal sealed class FakeTimeProvider : TimeProvider
{
    private readonly DateTimeOffset utcNow;

    public FakeTimeProvider(DateTimeOffset utcNow)
    {
        this.utcNow = utcNow;
    }

    public override DateTimeOffset GetUtcNow() => this.utcNow;
}
