namespace TownCrier.Infrastructure.Tests.Polling;

internal sealed class FakeTimeProvider : TimeProvider
{
    private DateTimeOffset utcNow;

    public void SetUtcNow(DateTimeOffset value) => this.utcNow = value;

    public void Advance(TimeSpan delta) => this.utcNow += delta;

    public override DateTimeOffset GetUtcNow() => this.utcNow;
}
