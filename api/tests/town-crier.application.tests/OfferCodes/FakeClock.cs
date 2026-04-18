namespace TownCrier.Application.Tests.OfferCodes;

internal sealed class FakeClock(DateTimeOffset utcNow) : TimeProvider
{
    public override DateTimeOffset GetUtcNow() => utcNow;
}
