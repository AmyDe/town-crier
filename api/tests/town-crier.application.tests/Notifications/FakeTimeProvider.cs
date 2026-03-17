namespace TownCrier.Application.Tests.Notifications;

internal sealed class FakeTimeProvider(DateTimeOffset utcNow) : TimeProvider
{
    private DateTimeOffset currentUtcNow = utcNow;

    public override DateTimeOffset GetUtcNow() => this.currentUtcNow;

    public void SetUtcNow(DateTimeOffset value)
    {
        this.currentUtcNow = value;
    }
}
