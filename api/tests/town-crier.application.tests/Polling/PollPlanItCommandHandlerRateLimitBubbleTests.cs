using System.Net;
using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.PlanIt;
using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

public sealed class PollPlanItCommandHandlerRateLimitBubbleTests
{
    [Test]
    public async Task Should_BubbleRetryAfter_When_PlanItClientThrowsRateLimit()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var planItClient = new FakePlanItClient();
        var retryAfter = TimeSpan.FromSeconds(75);
        planItClient.ThrowForAuthority(1, new PlanItRateLimitException(retryAfter));

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(result.RateLimited).IsTrue();
        await Assert.That(result.TerminationReason).IsEqualTo(PollTerminationReason.RateLimited);
        await Assert.That(result.RetryAfter).IsEqualTo(retryAfter);
    }

    [Test]
    public async Task Should_BubbleNullRetryAfter_When_RateLimitExceptionHasNoHeader()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var planItClient = new FakePlanItClient();
        planItClient.ThrowForAuthority(1, new PlanItRateLimitException(retryAfter: null));

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(result.RateLimited).IsTrue();
        await Assert.That(result.RetryAfter).IsNull();
    }

    [Test]
    public async Task Should_LeaveRetryAfterNull_When_NotRateLimited()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var planItClient = new FakePlanItClient();
        planItClient.Add(1, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(1).Build());

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(result.RateLimited).IsFalse();
        await Assert.That(result.RetryAfter).IsNull();
    }

    private static PollPlanItCommandHandler CreateHandler(
        FakePlanItClient planItClient,
        FakeActiveAuthorityProvider authorityProvider)
    {
        return new PollPlanItCommandHandler(
            planItClient,
            new FakePollStateStore(),
            new FakePlanningApplicationRepository(),
            TimeProvider.System,
            authorityProvider,
            new FakeWatchZoneRepository(),
            new FakeNotificationEnqueuer(),
            new FakeCycleSelector(CycleType.Watched),
            new PollingOptions(),
            new FakePollingLeaseStore { AcquireResult = true },
            NullLogger<PollPlanItCommandHandler>.Instance);
    }
}
