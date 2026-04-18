using TownCrier.Application.OfferCodes;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.OfferCodes;

public sealed class GenerateOfferCodesCommandHandlerTests
{
    [Test]
    public async Task Should_GenerateAndPersist_RequestedCount()
    {
        var repository = new FakeOfferCodeRepository();
        var generator = new FakeOfferCodeGenerator(
            "AAAAAAAAAAAA", "BBBBBBBBBBBB", "CCCCCCCCCCCC");
        var clock = new FakeClock(new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero));
        var handler = new GenerateOfferCodesCommandHandler(repository, generator, clock);

        var result = await handler.HandleAsync(
            new GenerateOfferCodesCommand(3, SubscriptionTier.Pro, 30),
            CancellationToken.None);

        await Assert.That(result.Codes).HasCount().EqualTo(3);
        await Assert.That(repository.Count).IsEqualTo(3);
        await Assert.That(repository.Snapshot().All(c => c.Tier == SubscriptionTier.Pro)).IsTrue();
        await Assert.That(repository.Snapshot().All(c => c.DurationDays == 30)).IsTrue();
    }

    [Test]
    [Arguments(0)]
    [Arguments(-5)]
    [Arguments(1001)]
    public async Task Should_Throw_When_CountOutOfRange(int count)
    {
        var handler = new GenerateOfferCodesCommandHandler(
            new FakeOfferCodeRepository(),
            new FakeOfferCodeGenerator(),
            new FakeClock(DateTimeOffset.UtcNow));

        await Assert.ThrowsAsync<ArgumentOutOfRangeException>(
            () => handler.HandleAsync(
                new GenerateOfferCodesCommand(count, SubscriptionTier.Pro, 30),
                CancellationToken.None));
    }

    [Test]
    public async Task Should_Throw_When_TierIsFree()
    {
        var handler = new GenerateOfferCodesCommandHandler(
            new FakeOfferCodeRepository(),
            new FakeOfferCodeGenerator(),
            new FakeClock(DateTimeOffset.UtcNow));

        await Assert.ThrowsAsync<ArgumentException>(
            () => handler.HandleAsync(
                new GenerateOfferCodesCommand(1, SubscriptionTier.Free, 30),
                CancellationToken.None));
    }

    [Test]
    [Arguments(0)]
    [Arguments(366)]
    public async Task Should_Throw_When_DurationOutOfRange(int days)
    {
        var handler = new GenerateOfferCodesCommandHandler(
            new FakeOfferCodeRepository(),
            new FakeOfferCodeGenerator(),
            new FakeClock(DateTimeOffset.UtcNow));

        await Assert.ThrowsAsync<ArgumentOutOfRangeException>(
            () => handler.HandleAsync(
                new GenerateOfferCodesCommand(1, SubscriptionTier.Pro, days),
                CancellationToken.None));
    }
}
