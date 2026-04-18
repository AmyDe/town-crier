namespace TownCrier.Application.Tests.OfferCodes;

internal sealed class FakeClock(DateTimeOffset now) : TimeProvider
{
    public override DateTimeOffset GetUtcNow() => now;
}
