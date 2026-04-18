using TownCrier.Application.OfferCodes;
using TownCrier.Application.Tests.Admin;
using TownCrier.Application.Tests.UserProfiles;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.OfferCodes;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.OfferCodes;

public sealed class RedeemOfferCodeCommandHandlerTests
{
    private const string UserId = "auth0|user-1";

    [Test]
    public async Task Should_ActivateSubscription_When_CodeValidAndUserFree()
    {
        var (handler, codeRepo, profileRepo, auth0) = BuildHandlerWithCode(
            code: "A7KMZQR3FNXP",
            tier: SubscriptionTier.Pro,
            durationDays: 30);
        await profileRepo.SaveAsync(
            UserProfile.Register(UserId, "user@example.com"),
            CancellationToken.None);

        var result = await handler.HandleAsync(
            new RedeemOfferCodeCommand(UserId, "A7KM-ZQR3-FNXP"),
            CancellationToken.None);

        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Pro);
        await Assert.That(result.ExpiresAt).IsEqualTo(new DateTimeOffset(2026, 5, 18, 12, 0, 0, TimeSpan.Zero));
        await Assert.That(profileRepo.GetByUserId(UserId)!.Tier).IsEqualTo(SubscriptionTier.Pro);
        await Assert.That(codeRepo.Snapshot().Single().IsRedeemed).IsTrue();
        await Assert.That(auth0.Updates).HasCount().EqualTo(1);
        await Assert.That(auth0.Updates[0].Tier).IsEqualTo("Pro");
    }

    [Test]
    public async Task Should_Throw_When_CodeFormatInvalid()
    {
        var (handler, _, _, _) = BuildHandlerWithCode("A7KMZQR3FNXP", SubscriptionTier.Pro, 30);

        await Assert.ThrowsAsync<InvalidOfferCodeFormatException>(
            () => handler.HandleAsync(
                new RedeemOfferCodeCommand(UserId, "not-a-code"),
                CancellationToken.None));
    }

    [Test]
    public async Task Should_Throw_When_CodeNotFound()
    {
        var (handler, _, profileRepo, _) = BuildHandlerWithCode("A7KMZQR3FNXP", SubscriptionTier.Pro, 30);
        await profileRepo.SaveAsync(
            UserProfile.Register(UserId, "user@example.com"),
            CancellationToken.None);

        await Assert.ThrowsAsync<OfferCodeNotFoundException>(
            () => handler.HandleAsync(
                new RedeemOfferCodeCommand(UserId, "BBBBBBBBBBBB"),
                CancellationToken.None));
    }

    [Test]
    public async Task Should_Throw_When_CodeAlreadyRedeemed()
    {
        var (handler, codeRepo, profileRepo, _) = BuildHandlerWithCode(
            "A7KMZQR3FNXP",
            SubscriptionTier.Pro,
            30);
        await profileRepo.SaveAsync(UserProfile.Register(UserId, "user@example.com"), CancellationToken.None);

        var existing = codeRepo.Snapshot().Single();
        existing.Redeem("auth0|other-user", DateTimeOffset.UtcNow);
        await codeRepo.SaveAsync(existing, CancellationToken.None);

        await Assert.ThrowsAsync<OfferCodeAlreadyRedeemedException>(
            () => handler.HandleAsync(
                new RedeemOfferCodeCommand(UserId, "A7KMZQR3FNXP"),
                CancellationToken.None));
    }

    [Test]
    public async Task Should_Throw_When_UserAlreadySubscribed()
    {
        var (handler, _, profileRepo, _) = BuildHandlerWithCode("A7KMZQR3FNXP", SubscriptionTier.Pro, 30);

        var profile = UserProfile.Register(UserId, "user@example.com");
        profile.ActivateSubscription(SubscriptionTier.Personal, DateTimeOffset.UtcNow.AddDays(30));
        await profileRepo.SaveAsync(profile, CancellationToken.None);

        await Assert.ThrowsAsync<AlreadySubscribedException>(
            () => handler.HandleAsync(
                new RedeemOfferCodeCommand(UserId, "A7KMZQR3FNXP"),
                CancellationToken.None));
    }

    [Test]
    public async Task Should_Throw_When_UserNotFound()
    {
        var (handler, _, _, _) = BuildHandlerWithCode("A7KMZQR3FNXP", SubscriptionTier.Pro, 30);

        await Assert.ThrowsAsync<UserProfileNotFoundException>(
            () => handler.HandleAsync(
                new RedeemOfferCodeCommand("auth0|missing", "A7KMZQR3FNXP"),
                CancellationToken.None));
    }

    private static (
        RedeemOfferCodeCommandHandler Handler,
        FakeOfferCodeRepository CodeRepo,
        FakeUserProfileRepository ProfileRepo,
        FakeAuth0ManagementClient Auth0) BuildHandlerWithCode(
            string code,
            SubscriptionTier tier,
            int durationDays)
    {
        var codeRepo = new FakeOfferCodeRepository();
        var offerCode = new OfferCode(
            code,
            tier,
            durationDays,
            new DateTimeOffset(2026, 4, 1, 0, 0, 0, TimeSpan.Zero));
        codeRepo.CreateAsync(offerCode, CancellationToken.None).GetAwaiter().GetResult();

        var profileRepo = new FakeUserProfileRepository();
        var auth0 = new FakeAuth0ManagementClient();
        var clock = new FakeClock(new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero));

        var handler = new RedeemOfferCodeCommandHandler(codeRepo, profileRepo, auth0, clock);
        return (handler, codeRepo, profileRepo, auth0);
    }
}
