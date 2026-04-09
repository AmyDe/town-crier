using System.Net;
using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

public sealed class PollPlanItCommandHandlerTests
{
    [Test]
    public async Task Should_ReturnApplicationCount_When_PlanItReturnsApplications()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var planItClient = new FakePlanItClient();
        planItClient.Add(1, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(1).Build());
        planItClient.Add(1, new PlanningApplicationBuilder().WithUid("app-2").WithAreaId(1).Build());
        planItClient.Add(1, new PlanningApplicationBuilder().WithUid("app-3").WithAreaId(1).Build());

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(result.ApplicationCount).IsEqualTo(3);
    }

    [Test]
    public async Task Should_ReturnZeroCount_When_NoActiveAuthorities()
    {
        var handler = CreateHandler();

        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(result.ApplicationCount).IsEqualTo(0);
    }

    [Test]
    public async Task Should_NotCallPlanItClient_When_NoActiveAuthorities()
    {
        var planItClient = new FakePlanItClient();
        var handler = CreateHandler(planItClient: planItClient);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(planItClient.AuthorityIdsRequested).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_FetchForAllActiveAuthorities()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.Add(200, new PlanningApplicationBuilder().WithUid("app-2").WithAreaId(200).Build());

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(result.ApplicationCount).IsEqualTo(2);
        await Assert.That(result.AuthoritiesPolled).IsEqualTo(2);
        await Assert.That(planItClient.AuthorityIdsRequested).Contains(100);
        await Assert.That(planItClient.AuthorityIdsRequested).Contains(200);
    }

    [Test]
    public async Task Should_UseDefault30DayLookback_When_NoPreviousPollState()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var planItClient = new FakePlanItClient();
        var now = new DateTimeOffset(2026, 4, 5, 12, 0, 0, TimeSpan.Zero);
        var handler = CreateHandler(
            planItClient: planItClient,
            authorityProvider: authorityProvider,
            timeProvider: new FakeTimeProvider(now));

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        var expected = new DateTimeOffset(2026, 3, 6, 12, 0, 0, TimeSpan.Zero);
        await Assert.That(planItClient.LastDifferentStartUsed).IsEqualTo(expected);
    }

    [Test]
    public async Task Should_PassLastPollTime_When_PreviousPollStateExists()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var planItClient = new FakePlanItClient();
        var pollStateStore = new FakePollStateStore();
        var lastPoll = new DateTimeOffset(2026, 3, 15, 10, 0, 0, TimeSpan.Zero);
        pollStateStore.SetLastPollTime(1, lastPoll);

        var handler = CreateHandler(planItClient: planItClient, pollStateStore: pollStateStore, authorityProvider: authorityProvider);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(planItClient.LastDifferentStartUsed).IsEqualTo(lastPoll);
    }

    [Test]
    public async Task Should_PersistCurrentTime_When_PollSucceeds()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var pollStateStore = new FakePollStateStore();
        var fakeTime = new DateTimeOffset(2026, 3, 16, 12, 0, 0, TimeSpan.Zero);
        var handler = CreateHandler(
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider,
            timeProvider: new FakeTimeProvider(fakeTime));

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(pollStateStore.GetLastPollTimeFor(1)).IsEqualTo(fakeTime);
        await Assert.That(pollStateStore.SaveCallCount).IsEqualTo(1);
    }

    [Test]
    public async Task Should_UpsertAllApplications_When_PlanItReturnsResults()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var planItClient = new FakePlanItClient();
        planItClient.Add(1, new PlanningApplicationBuilder().WithUid("app-1").WithName("Council/app-1").WithAreaId(1).Build());
        planItClient.Add(1, new PlanningApplicationBuilder().WithUid("app-2").WithName("Council/app-2").WithAreaId(1).Build());

        var repository = new FakePlanningApplicationRepository();
        var handler = CreateHandler(planItClient: planItClient, repository: repository, authorityProvider: authorityProvider);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(repository.GetAll()).HasCount().EqualTo(2);
    }

    [Test]
    public async Task Should_EnqueueNotification_When_ApplicationIsWithinWatchZone()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var watchZoneRepository = new FakeWatchZoneRepository();
        watchZoneRepository.Add(new WatchZoneBuilder()
            .WithId("zone-1").WithUserId("user-1")
            .WithCentre(51.5074, -0.1278).WithRadiusMetres(5000).WithAuthorityId(1).Build());

        var notificationEnqueuer = new FakeNotificationEnqueuer();
        var planItClient = new FakePlanItClient();
        planItClient.Add(1, new PlanningApplicationBuilder()
            .WithUid("app-1").WithAreaId(1).WithCoordinates(51.5080, -0.1270).Build());

        var handler = CreateHandler(
            planItClient: planItClient,
            authorityProvider: authorityProvider,
            watchZoneRepository: watchZoneRepository,
            notificationEnqueuer: notificationEnqueuer);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(notificationEnqueuer.Enqueued).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_NotEnqueueNotification_When_ApplicationHasNoCoordinates()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var watchZoneRepository = new FakeWatchZoneRepository();
        watchZoneRepository.Add(new WatchZoneBuilder()
            .WithId("zone-1").WithUserId("user-1")
            .WithCentre(51.5074, -0.1278).WithRadiusMetres(5000).WithAuthorityId(1).Build());

        var notificationEnqueuer = new FakeNotificationEnqueuer();
        var planItClient = new FakePlanItClient();
        planItClient.Add(1, new PlanningApplicationBuilder().WithUid("app-no-coords").WithAreaId(1).Build());

        var handler = CreateHandler(
            planItClient: planItClient,
            authorityProvider: authorityProvider,
            watchZoneRepository: watchZoneRepository,
            notificationEnqueuer: notificationEnqueuer);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(notificationEnqueuer.Enqueued).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_SavePollStateAfterEachAuthority_When_MultipleAuthoritiesPolled()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);
        authorityProvider.Add(300);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.Add(200, new PlanningApplicationBuilder().WithUid("app-2").WithAreaId(200).Build());
        planItClient.Add(300, new PlanningApplicationBuilder().WithUid("app-3").WithAreaId(300).Build());

        var pollStateStore = new FakePollStateStore();
        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(pollStateStore.SaveCallCount).IsEqualTo(3);
    }

    [Test]
    public async Task Should_ReturnZeroApplications_When_SingleAuthorityFails()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);
        var failingClient = new FakePlanItClient { ExceptionToThrow = new HttpRequestException("timeout") };

        var handler = CreateHandler(planItClient: failingClient, authorityProvider: authorityProvider);

        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(result.ApplicationCount).IsEqualTo(0);
        await Assert.That(result.AuthoritiesPolled).IsEqualTo(0);
    }

    [Test]
    public async Task Should_ContinueAndPreserveProgress_When_MiddleAuthorityFails()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);
        authorityProvider.Add(300);

        var app100LastDifferent = new DateTimeOffset(2026, 4, 4, 10, 0, 0, TimeSpan.Zero);
        var app300LastDifferent = new DateTimeOffset(2026, 4, 4, 14, 0, 0, TimeSpan.Zero);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder()
            .WithUid("app-1").WithAreaId(100).WithLastDifferent(app100LastDifferent).Build());
        planItClient.Add(300, new PlanningApplicationBuilder()
            .WithUid("app-3").WithAreaId(300).WithLastDifferent(app300LastDifferent).Build());
        planItClient.ThrowForAuthority(200, new HttpRequestException("rate limited"));

        var pollStateStore = new FakePollStateStore();
        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider);

        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Authority 100 and 300 completed, 200 failed but was isolated
        await Assert.That(result.ApplicationCount).IsEqualTo(2);
        await Assert.That(result.AuthoritiesPolled).IsEqualTo(2);
        await Assert.That(pollStateStore.SaveCallCount).IsEqualTo(2);
        await Assert.That(pollStateStore.GetLastPollTimeFor(100)).IsEqualTo(app100LastDifferent);
        await Assert.That(pollStateStore.GetLastPollTimeFor(300)).IsEqualTo(app300LastDifferent);
        await Assert.That(pollStateStore.GetLastPollTimeFor(200)).IsNull();
    }

    [Test]
    public async Task Should_NotSavePollState_When_OnlyAuthorityFails()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);
        var failingClient = new FakePlanItClient { ExceptionToThrow = new HttpRequestException("timeout") };
        var pollStateStore = new FakePollStateStore();

        var handler = CreateHandler(
            planItClient: failingClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(pollStateStore.GetLastPollTimeFor(1)).IsNull();
    }

    [Test]
    public async Task Should_BreakImmediately_When_RateLimitHit()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);
        authorityProvider.Add(300);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.ThrowForAuthority(200, new HttpRequestException("Rate limited", null, HttpStatusCode.TooManyRequests));
        planItClient.Add(300, new PlanningApplicationBuilder().WithUid("app-3").WithAreaId(300).Build());

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Only authority 100 completed before the 429 stopped the loop
        await Assert.That(result.ApplicationCount).IsEqualTo(1);
        await Assert.That(result.AuthoritiesPolled).IsEqualTo(1);

        // Authority 300 was never attempted
        await Assert.That(planItClient.AuthorityIdsRequested).DoesNotContain(300);
    }

    [Test]
    public async Task Should_ContinueToNextAuthority_When_NonRateLimitExceptionOccurs()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);
        authorityProvider.Add(300);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.ThrowForAuthority(200, new InvalidOperationException("Unexpected JSON structure"));
        planItClient.Add(300, new PlanningApplicationBuilder().WithUid("app-3").WithAreaId(300).Build());

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Authority 200 failed but 100 and 300 should still succeed
        await Assert.That(result.ApplicationCount).IsEqualTo(2);
        await Assert.That(result.AuthoritiesPolled).IsEqualTo(2);
        await Assert.That(planItClient.AuthorityIdsRequested).Contains(300);
    }

    [Test]
    public async Task Should_ContinueToNextAuthority_When_NonRateLimitHttpErrorOccurs()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);
        authorityProvider.Add(300);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.ThrowForAuthority(200, new HttpRequestException("Internal Server Error", null, HttpStatusCode.InternalServerError));
        planItClient.Add(300, new PlanningApplicationBuilder().WithUid("app-3").WithAreaId(300).Build());

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(result.ApplicationCount).IsEqualTo(2);
        await Assert.That(result.AuthoritiesPolled).IsEqualTo(2);
    }

    [Test]
    public async Task Should_Use30DayLookback_When_NewAuthorityHasNoPollState()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);

        var pollStateStore = new FakePollStateStore();
        var existingPollTime = new DateTimeOffset(2026, 4, 5, 10, 0, 0, TimeSpan.Zero);
        pollStateStore.SetLastPollTime(100, existingPollTime);

        var now = new DateTimeOffset(2026, 4, 5, 12, 0, 0, TimeSpan.Zero);
        var planItClient = new FakePlanItClient();

        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider,
            timeProvider: new FakeTimeProvider(now));

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Authority 100 should use its existing poll time
        await Assert.That(planItClient.DifferentStartByAuthority[100]).IsEqualTo(existingPollTime);

        // Authority 200 should use 30-day lookback
        var expected30DaysAgo = new DateTimeOffset(2026, 3, 6, 12, 0, 0, TimeSpan.Zero);
        await Assert.That(planItClient.DifferentStartByAuthority[200]).IsEqualTo(expected30DaysAgo);
    }

    [Test]
    public async Task Should_RetainPerAuthorityPollTime_When_AuthorityIsRateLimited()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);

        var pollStateStore = new FakePollStateStore();
        var authority100Time = new DateTimeOffset(2026, 4, 3, 8, 0, 0, TimeSpan.Zero);
        var authority200Time = new DateTimeOffset(2026, 4, 4, 10, 0, 0, TimeSpan.Zero);
        pollStateStore.SetLastPollTime(100, authority100Time);
        pollStateStore.SetLastPollTime(200, authority200Time);

        var app100LastDifferent = new DateTimeOffset(2026, 4, 5, 10, 0, 0, TimeSpan.Zero);
        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder()
            .WithUid("app-1").WithAreaId(100).WithLastDifferent(app100LastDifferent).Build());
        planItClient.ThrowForAuthority(200, new HttpRequestException("Rate limited", null, HttpStatusCode.TooManyRequests));

        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Authority 100 should be advanced to high-water mark
        await Assert.That(pollStateStore.GetLastPollTimeFor(100)).IsEqualTo(app100LastDifferent);

        // Authority 200 should retain its original poll time (rate limited, not advanced)
        await Assert.That(pollStateStore.GetLastPollTimeFor(200)).IsEqualTo(authority200Time);
    }

    [Test]
    public async Task Should_DeleteGlobalPollState_When_CycleCompletes()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var pollStateStore = new FakePollStateStore();
        var handler = CreateHandler(pollStateStore: pollStateStore, authorityProvider: authorityProvider);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(pollStateStore.DeleteGlobalCalled).IsTrue();
    }

    [Test]
    public async Task Should_DeleteGlobalPollState_When_NoActiveAuthorities()
    {
        var pollStateStore = new FakePollStateStore();
        var handler = CreateHandler(pollStateStore: pollStateStore);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(pollStateStore.DeleteGlobalCalled).IsTrue();
    }

    [Test]
    public async Task Should_StopAndSetRateLimited_When_429Hit()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);
        authorityProvider.Add(300);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.ThrowForAuthority(200, new HttpRequestException("Rate limited", null, HttpStatusCode.TooManyRequests));
        planItClient.Add(300, new PlanningApplicationBuilder().WithUid("app-3").WithAreaId(300).Build());

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Authority 100 completed, 200 triggered 429, 300 never attempted
        await Assert.That(result.ApplicationCount).IsEqualTo(1);
        await Assert.That(result.AuthoritiesPolled).IsEqualTo(1);
        await Assert.That(result.RateLimited).IsTrue();
        await Assert.That(planItClient.AuthorityIdsRequested).DoesNotContain(300);
    }

    [Test]
    public async Task Should_NotSetRateLimited_When_NoRateLimitHit()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(result.RateLimited).IsFalse();
    }

    [Test]
    public async Task Should_PollLeastRecentlyPolledFirst_When_MultipleAuthorities()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);
        authorityProvider.Add(300);

        var pollStateStore = new FakePollStateStore();

        // Authority 300 polled longest ago, 200 most recently, 100 in between
        pollStateStore.SetLastPollTime(300, new DateTimeOffset(2026, 4, 1, 0, 0, 0, TimeSpan.Zero));
        pollStateStore.SetLastPollTime(100, new DateTimeOffset(2026, 4, 3, 0, 0, 0, TimeSpan.Zero));
        pollStateStore.SetLastPollTime(200, new DateTimeOffset(2026, 4, 5, 0, 0, 0, TimeSpan.Zero));

        var planItClient = new FakePlanItClient();

        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Should be polled in order: 300 (oldest), 100, 200 (newest)
        await Assert.That(planItClient.AuthorityIdsRequested).HasCount().EqualTo(3);
        await Assert.That(planItClient.AuthorityIdsRequested[0]).IsEqualTo(300);
        await Assert.That(planItClient.AuthorityIdsRequested[1]).IsEqualTo(100);
        await Assert.That(planItClient.AuthorityIdsRequested[2]).IsEqualTo(200);
    }

    [Test]
    public async Task Should_PollNeverPolledAuthorityFirst_When_MixedPollState()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);

        var pollStateStore = new FakePollStateStore();
        pollStateStore.SetLastPollTime(100, new DateTimeOffset(2026, 4, 5, 0, 0, 0, TimeSpan.Zero));

        var planItClient = new FakePlanItClient();

        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Never-polled authority 200 should be first
        await Assert.That(planItClient.AuthorityIdsRequested[0]).IsEqualTo(200);
    }

    [Test]
    public async Task Should_SavePollState_When_429HitAfterSomeApplicationsIngested()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var planItClient = new FakePlanItClient();
        var app1 = new PlanningApplicationBuilder()
            .WithUid("app-1").WithAreaId(100)
            .WithLastDifferent(new DateTimeOffset(2026, 3, 10, 0, 0, 0, TimeSpan.Zero))
            .Build();
        var app2 = new PlanningApplicationBuilder()
            .WithUid("app-2").WithAreaId(100)
            .WithLastDifferent(new DateTimeOffset(2026, 3, 12, 0, 0, 0, TimeSpan.Zero))
            .Build();
        var app3 = new PlanningApplicationBuilder()
            .WithUid("app-3").WithAreaId(100)
            .WithLastDifferent(new DateTimeOffset(2026, 3, 14, 0, 0, 0, TimeSpan.Zero))
            .Build();
        planItClient.Add(100, app1);
        planItClient.Add(100, app2);
        planItClient.Add(100, app3);
        planItClient.ThrowAfterYielding(
            100,
            2,
            new HttpRequestException("Rate limited", null, HttpStatusCode.TooManyRequests));

        var pollStateStore = new FakePollStateStore();
        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(pollStateStore.GetLastPollTimeFor(100)).IsNotNull();
        await Assert.That(pollStateStore.SaveCallCount).IsEqualTo(1);
    }

    [Test]
    public async Task Should_EmitApplicationsIngested_When_429HitMidPagination()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-2").WithAreaId(100).Build());
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-3").WithAreaId(100).Build());
        planItClient.ThrowAfterYielding(
            100,
            2,
            new HttpRequestException("Rate limited", null, HttpStatusCode.TooManyRequests));

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(result.ApplicationCount).IsEqualTo(2);
        await Assert.That(result.RateLimited).IsTrue();
    }

    [Test]
    public async Task Should_EmitAuthoritiesPolled_When_429HitMidPagination()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-2").WithAreaId(100).Build());
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-3").WithAreaId(100).Build());
        planItClient.ThrowAfterYielding(
            100,
            2,
            new HttpRequestException("Rate limited", null, HttpStatusCode.TooManyRequests));

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(result.AuthoritiesPolled).IsEqualTo(1);
    }

    [Test]
    public async Task Should_AdvanceToNextAuthority_When_PreviousAuthorityPartiallyCompleted()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-2").WithAreaId(100).Build());
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-3").WithAreaId(100).Build());

        // Mid-pagination 429 on authority 100 after 2 apps
        planItClient.ThrowAfterYielding(
            100,
            2,
            new HttpRequestException("Rate limited", null, HttpStatusCode.TooManyRequests));
        planItClient.Add(200, new PlanningApplicationBuilder().WithUid("app-4").WithAreaId(200).Build());

        var pollStateStore = new FakePollStateStore();
        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider);

        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(result.ApplicationCount).IsEqualTo(2);

        await Assert.That(pollStateStore.GetLastPollTimeFor(100)).IsNotNull();

        await Assert.That(result.RateLimited).IsTrue();
    }

    private static PollPlanItCommandHandler CreateHandler(
        FakePlanItClient? planItClient = null,
        FakePollStateStore? pollStateStore = null,
        FakePlanningApplicationRepository? repository = null,
        FakeActiveAuthorityProvider? authorityProvider = null,
        FakeWatchZoneRepository? watchZoneRepository = null,
        FakeNotificationEnqueuer? notificationEnqueuer = null,
        TimeProvider? timeProvider = null)
    {
        return new PollPlanItCommandHandler(
            planItClient ?? new FakePlanItClient(),
            pollStateStore ?? new FakePollStateStore(),
            repository ?? new FakePlanningApplicationRepository(),
            timeProvider ?? TimeProvider.System,
            authorityProvider ?? new FakeActiveAuthorityProvider(),
            watchZoneRepository ?? new FakeWatchZoneRepository(),
            notificationEnqueuer ?? new FakeNotificationEnqueuer(),
            NullLogger<PollPlanItCommandHandler>.Instance);
    }
}
