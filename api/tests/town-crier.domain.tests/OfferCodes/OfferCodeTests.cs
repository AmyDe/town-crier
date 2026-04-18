using TownCrier.Domain.OfferCodes;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Domain.Tests.OfferCodes;

public sealed class OfferCodeTests
{
    [Test]
    public async Task Should_Construct_When_AllInputsValid()
    {
        var created = new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero);
        var code = new OfferCode("A7KMZQR3FNXP", SubscriptionTier.Pro, 30, created);

        await Assert.That(code.Code).IsEqualTo("A7KMZQR3FNXP");
        await Assert.That(code.Tier).IsEqualTo(SubscriptionTier.Pro);
        await Assert.That(code.DurationDays).IsEqualTo(30);
        await Assert.That(code.CreatedAt).IsEqualTo(created);
        await Assert.That(code.RedeemedByUserId).IsNull();
        await Assert.That(code.RedeemedAt).IsNull();
        await Assert.That(code.IsRedeemed).IsFalse();
    }

    [Test]
    public async Task Should_Throw_When_TierIsFree()
    {
        await Assert.ThrowsAsync<ArgumentException>(
            () => Task.FromResult(new OfferCode("A7KMZQR3FNXP", SubscriptionTier.Free, 30, DateTimeOffset.UtcNow)));
    }

    [Test]
    [Arguments(0)]
    [Arguments(-1)]
    [Arguments(366)]
    public async Task Should_Throw_When_DurationOutOfRange(int duration)
    {
        await Assert.ThrowsAsync<ArgumentOutOfRangeException>(
            () => Task.FromResult(new OfferCode("A7KMZQR3FNXP", SubscriptionTier.Pro, duration, DateTimeOffset.UtcNow)));
    }

    [Test]
    [Arguments("SHORT")] // too short
    [Arguments("A7KMZQR3FNXPTOOLONG")] // too long
    [Arguments("a7kmzqr3fnxp")] // lowercase
    [Arguments("A7KM-ZQR3-FNXP")] // has separators
    [Arguments("A7KMZQR3FNXI")] // contains excluded letter I
    public async Task Should_Throw_When_CodeMalformed(string code)
    {
        await Assert.ThrowsAsync<ArgumentException>(
            () => Task.FromResult(new OfferCode(code, SubscriptionTier.Pro, 30, DateTimeOffset.UtcNow)));
    }

    [Test]
    public async Task Should_RecordRedemption_When_RedeemCalled()
    {
        var now = new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero);
        var code = new OfferCode("A7KMZQR3FNXP", SubscriptionTier.Pro, 30, now.AddDays(-1));

        code.Redeem("auth0|user-1", now);

        await Assert.That(code.RedeemedByUserId).IsEqualTo("auth0|user-1");
        await Assert.That(code.RedeemedAt).IsEqualTo(now);
        await Assert.That(code.IsRedeemed).IsTrue();
    }

    [Test]
    public async Task Should_Throw_When_RedeemCalledTwice()
    {
        var now = DateTimeOffset.UtcNow;
        var code = new OfferCode("A7KMZQR3FNXP", SubscriptionTier.Pro, 30, now);
        code.Redeem("auth0|user-1", now);

        await Assert.ThrowsAsync<OfferCodeAlreadyRedeemedException>(
            () =>
            {
                code.Redeem("auth0|user-2", now);
                return Task.CompletedTask;
            });
    }
}
