namespace TownCrier.Application.Tests.DeviceRegistrations;

internal sealed class FakeTimeProvider(DateTimeOffset utcNow) : TimeProvider
{
    private DateTimeOffset currentTime = utcNow;

    public override DateTimeOffset GetUtcNow() => this.currentTime;

    public void Advance(TimeSpan duration) => this.currentTime = this.currentTime.Add(duration);
}
