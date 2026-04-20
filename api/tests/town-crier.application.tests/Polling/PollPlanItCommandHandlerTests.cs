using System.Net;
using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

public sealed class PollPlanItCommandHandlerTests
{
    private static readonly int[] ExpectedPages1Through3 = [1, 2, 3];
    private static readonly int[] ExpectedPagesAcrossThreeCycles = [1, 2, 3, 3, 4, 5, 5, 6, 7];

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
    public async Task Should_UseDefault1DayLookback_When_NoPreviousPollState()
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

        var expected = new DateTimeOffset(2026, 4, 4, 12, 0, 0, TimeSpan.Zero);
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
    public async Task Should_NotEnqueueNotification_When_ZoneCreatedAfterApplicationLastDifferent()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var applicationLastDifferent = new DateTimeOffset(2026, 3, 10, 12, 0, 0, TimeSpan.Zero);
        var zoneCreatedAt = new DateTimeOffset(2026, 3, 15, 14, 0, 0, TimeSpan.Zero);

        var watchZoneRepository = new FakeWatchZoneRepository();
        watchZoneRepository.Add(new WatchZoneBuilder()
            .WithId("zone-1").WithUserId("user-1")
            .WithCentre(51.5074, -0.1278).WithRadiusMetres(5000).WithAuthorityId(1)
            .WithCreatedAt(zoneCreatedAt).Build());

        var notificationEnqueuer = new FakeNotificationEnqueuer();
        var planItClient = new FakePlanItClient();
        planItClient.Add(1, new PlanningApplicationBuilder()
            .WithUid("app-1").WithAreaId(1).WithCoordinates(51.5080, -0.1270)
            .WithLastDifferent(applicationLastDifferent).Build());

        var handler = CreateHandler(
            planItClient: planItClient,
            authorityProvider: authorityProvider,
            watchZoneRepository: watchZoneRepository,
            notificationEnqueuer: notificationEnqueuer);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(notificationEnqueuer.Enqueued).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_EnqueueNotification_When_ZoneCreatedBeforeApplicationLastDifferent()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var applicationLastDifferent = new DateTimeOffset(2026, 3, 15, 14, 0, 0, TimeSpan.Zero);
        var zoneCreatedAt = new DateTimeOffset(2026, 3, 10, 12, 0, 0, TimeSpan.Zero);

        var watchZoneRepository = new FakeWatchZoneRepository();
        watchZoneRepository.Add(new WatchZoneBuilder()
            .WithId("zone-1").WithUserId("user-1")
            .WithCentre(51.5074, -0.1278).WithRadiusMetres(5000).WithAuthorityId(1)
            .WithCreatedAt(zoneCreatedAt).Build());

        var notificationEnqueuer = new FakeNotificationEnqueuer();
        var planItClient = new FakePlanItClient();
        planItClient.Add(1, new PlanningApplicationBuilder()
            .WithUid("app-1").WithAreaId(1).WithCoordinates(51.5080, -0.1270)
            .WithLastDifferent(applicationLastDifferent).Build());

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
    public async Task Should_Use1DayLookback_When_NewAuthorityHasNoPollState()
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

        // Authority 200 should use 1-day lookback
        var expected1DayAgo = new DateTimeOffset(2026, 4, 4, 12, 0, 0, TimeSpan.Zero);
        await Assert.That(planItClient.DifferentStartByAuthority[200]).IsEqualTo(expected1DayAgo);
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

    [Test]
    public async Task Should_SkipUpsert_When_SameApplicationReturnedTwiceWithUnchangedBusinessFields()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var existing = new PlanningApplicationBuilder()
            .WithUid("app-1").WithName("Council/app-1").WithAreaId(1)
            .WithLastDifferent(new DateTimeOffset(2026, 3, 1, 0, 0, 0, TimeSpan.Zero))
            .Build();

        var repository = new FakePlanningApplicationRepository();
        await repository.UpsertAsync(existing, CancellationToken.None);
        var upsertCountBeforePoll = repository.UpsertCallCount;

        // Same business fields but new last_different (simulating PlanIt rescrape bump)
        var rescraped = new PlanningApplicationBuilder()
            .WithUid("app-1").WithName("Council/app-1").WithAreaId(1)
            .WithLastDifferent(new DateTimeOffset(2026, 4, 10, 0, 0, 0, TimeSpan.Zero))
            .Build();

        var planItClient = new FakePlanItClient();
        planItClient.Add(1, rescraped);

        var handler = CreateHandler(
            planItClient: planItClient,
            repository: repository,
            authorityProvider: authorityProvider);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(repository.UpsertCallCount).IsEqualTo(upsertCountBeforePoll);
    }

    [Test]
    public async Task Should_SkipZoneLookup_When_SameApplicationReturnedTwiceWithUnchangedBusinessFields()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var existing = new PlanningApplicationBuilder()
            .WithUid("app-1").WithName("Council/app-1").WithAreaId(1)
            .WithCoordinates(51.5074, -0.1278)
            .WithLastDifferent(new DateTimeOffset(2026, 3, 1, 0, 0, 0, TimeSpan.Zero))
            .Build();

        var repository = new FakePlanningApplicationRepository();
        await repository.UpsertAsync(existing, CancellationToken.None);

        var rescraped = new PlanningApplicationBuilder()
            .WithUid("app-1").WithName("Council/app-1").WithAreaId(1)
            .WithCoordinates(51.5074, -0.1278)
            .WithLastDifferent(new DateTimeOffset(2026, 4, 10, 0, 0, 0, TimeSpan.Zero))
            .Build();

        var planItClient = new FakePlanItClient();
        planItClient.Add(1, rescraped);

        var watchZoneRepository = new FakeWatchZoneRepository();
        watchZoneRepository.Add(new WatchZoneBuilder()
            .WithId("zone-1").WithUserId("user-1")
            .WithCentre(51.5074, -0.1278).WithRadiusMetres(5000).WithAuthorityId(1).Build());

        var notificationEnqueuer = new FakeNotificationEnqueuer();

        var handler = CreateHandler(
            planItClient: planItClient,
            repository: repository,
            authorityProvider: authorityProvider,
            watchZoneRepository: watchZoneRepository,
            notificationEnqueuer: notificationEnqueuer);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(watchZoneRepository.FindZonesContainingCallCount).IsEqualTo(0);
        await Assert.That(notificationEnqueuer.Enqueued).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_UpsertAndFanOut_When_BusinessFieldsChanged()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var existing = new PlanningApplicationBuilder()
            .WithUid("app-1").WithName("Council/app-1").WithAreaId(1)
            .WithAppState("Undecided")
            .WithCoordinates(51.5074, -0.1278)
            .WithLastDifferent(new DateTimeOffset(2026, 3, 1, 0, 0, 0, TimeSpan.Zero))
            .Build();

        var repository = new FakePlanningApplicationRepository();
        await repository.UpsertAsync(existing, CancellationToken.None);
        var upsertsBefore = repository.UpsertCallCount;

        // AppState changed — this IS a material change
        var updated = new PlanningApplicationBuilder()
            .WithUid("app-1").WithName("Council/app-1").WithAreaId(1)
            .WithAppState("Decided")
            .WithCoordinates(51.5074, -0.1278)
            .WithLastDifferent(new DateTimeOffset(2026, 4, 10, 0, 0, 0, TimeSpan.Zero))
            .Build();

        var planItClient = new FakePlanItClient();
        planItClient.Add(1, updated);

        var watchZoneRepository = new FakeWatchZoneRepository();
        watchZoneRepository.Add(new WatchZoneBuilder()
            .WithId("zone-1").WithUserId("user-1")
            .WithCentre(51.5074, -0.1278).WithRadiusMetres(5000).WithAuthorityId(1).Build());

        var notificationEnqueuer = new FakeNotificationEnqueuer();

        var handler = CreateHandler(
            planItClient: planItClient,
            repository: repository,
            authorityProvider: authorityProvider,
            watchZoneRepository: watchZoneRepository,
            notificationEnqueuer: notificationEnqueuer);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(repository.UpsertCallCount).IsEqualTo(upsertsBefore + 1);
        await Assert.That(watchZoneRepository.FindZonesContainingCallCount).IsEqualTo(1);
        await Assert.That(notificationEnqueuer.Enqueued).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_UpsertAndFanOut_When_NoExistingApplication()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var application = new PlanningApplicationBuilder()
            .WithUid("app-1").WithName("Council/app-1").WithAreaId(1)
            .WithCoordinates(51.5074, -0.1278)
            .Build();

        var planItClient = new FakePlanItClient();
        planItClient.Add(1, application);

        var repository = new FakePlanningApplicationRepository();
        var watchZoneRepository = new FakeWatchZoneRepository();
        watchZoneRepository.Add(new WatchZoneBuilder()
            .WithId("zone-1").WithUserId("user-1")
            .WithCentre(51.5074, -0.1278).WithRadiusMetres(5000).WithAuthorityId(1).Build());

        var notificationEnqueuer = new FakeNotificationEnqueuer();

        var handler = CreateHandler(
            planItClient: planItClient,
            repository: repository,
            authorityProvider: authorityProvider,
            watchZoneRepository: watchZoneRepository,
            notificationEnqueuer: notificationEnqueuer);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(repository.UpsertCallCount).IsEqualTo(1);
        await Assert.That(watchZoneRepository.FindZonesContainingCallCount).IsEqualTo(1);
        await Assert.That(notificationEnqueuer.Enqueued).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_SetTerminationNatural_When_AllAuthoritiesProcessed()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.Add(200, new PlanningApplicationBuilder().WithUid("app-2").WithAreaId(200).Build());

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(result.TerminationReason).IsEqualTo(PollTerminationReason.Natural);
        await Assert.That(result.AuthorityErrors).IsEqualTo(0);
    }

    [Test]
    public async Task Should_SetTerminationTimeBounded_When_CancellationRequestedMidLoop()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);
        authorityProvider.Add(300);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.Add(200, new PlanningApplicationBuilder().WithUid("app-2").WithAreaId(200).Build());
        planItClient.Add(300, new PlanningApplicationBuilder().WithUid("app-3").WithAreaId(300).Build());

        using var cts = new CancellationTokenSource();
        var pollStateStore = new FakePollStateStore
        {
            OnSave = (_, _) => cts.Cancel(),
        };

        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider);

        var result = await handler.HandleAsync(new PollPlanItCommand(), cts.Token);

        await Assert.That(result.TerminationReason).IsEqualTo(PollTerminationReason.TimeBounded);
    }

    [Test]
    public async Task Should_SetTerminationRateLimited_When_429Hit()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var planItClient = new FakePlanItClient();
        planItClient.ThrowForAuthority(100, new HttpRequestException("Rate limited", null, HttpStatusCode.TooManyRequests));

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(result.TerminationReason).IsEqualTo(PollTerminationReason.RateLimited);
    }

    [Test]
    public async Task Should_CountAuthorityErrors_When_NonRateLimitExceptionsOccur()
    {
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);
        authorityProvider.Add(300);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.ThrowForAuthority(200, new HttpRequestException("Internal Server Error", null, HttpStatusCode.InternalServerError));
        planItClient.ThrowForAuthority(300, new InvalidOperationException("bad JSON"));

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Two per-authority errors (500 + InvalidOp), authority 100 succeeded with one app.
        await Assert.That(result.AuthorityErrors).IsEqualTo(2);
        await Assert.That(result.ApplicationCount).IsEqualTo(1);
        await Assert.That(result.TerminationReason).IsEqualTo(PollTerminationReason.Natural);
    }

    [Test]
    public async Task Should_BreakLoopAndReturnPartial_When_CancellationRequestedMidLoop()
    {
        // Arrange — three authorities, cancel after the first completes so only one is polled.
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);
        authorityProvider.Add(300);

        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.Add(200, new PlanningApplicationBuilder().WithUid("app-2").WithAreaId(200).Build());
        planItClient.Add(300, new PlanningApplicationBuilder().WithUid("app-3").WithAreaId(300).Build());

        using var cts = new CancellationTokenSource();

        // Cancel as soon as the first authority's poll state is saved — simulating the
        // infra replicaTimeout firing mid-loop between authorities.
        var pollStateStore = new FakePollStateStore
        {
            OnSave = (_, _) => cts.Cancel(),
        };

        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider);

        // Act — must NOT throw, must return the partial PollPlanItResult.
        var result = await handler.HandleAsync(new PollPlanItCommand(), cts.Token);

        // Assert — only authority 100 was polled; 200 and 300 skipped cleanly.
        await Assert.That(result.ApplicationCount).IsEqualTo(1);
        await Assert.That(result.AuthoritiesPolled).IsEqualTo(1);
        await Assert.That(result.RateLimited).IsFalse();
        await Assert.That(planItClient.AuthorityIdsRequested).DoesNotContain(200);
        await Assert.That(planItClient.AuthorityIdsRequested).DoesNotContain(300);
    }

    [Test]
    public async Task Should_StopFetchingAtConfiguredMaxPages_When_PollingAuthority()
    {
        // Handler now drives pagination itself: seed the fake with 5 full pages
        // and verify the handler stops after MaxPagesPerAuthorityPerCycle pages.
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var planItClient = new FakePlanItClient();
        for (var i = 0; i < FakePlanItClient.PageSize * 5; i++)
        {
            planItClient.Add(1, new PlanningApplicationBuilder().WithUid($"app-{i}").WithAreaId(1).Build());
        }

        var options = new PollingOptions { MaxPagesPerAuthorityPerCycle = 3 };

        var handler = CreateHandler(
            planItClient: planItClient,
            authorityProvider: authorityProvider,
            options: options);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Exactly 3 page requests for authority 1, pages 1..3.
        var pages = planItClient.PagesRequested.Where(p => p.AuthorityId == 1).Select(p => p.Page).ToList();
        await Assert.That(pages).IsEquivalentTo(ExpectedPages1Through3);
    }

    [Test]
    public async Task Should_PaginateUnbounded_When_PollingOptionsUnset()
    {
        // Regression guard: default PollingOptions (MaxPagesPerAuthorityPerCycle = null)
        // must allow pagination to continue until HasMorePages=false so watched-cycle
        // callers don't prematurely truncate results.
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var planItClient = new FakePlanItClient();

        // Seed 2.5 pages worth — page 3 will be partial so HasMorePages flips false.
        for (var i = 0; i < (FakePlanItClient.PageSize * 2) + 50; i++)
        {
            planItClient.Add(1, new PlanningApplicationBuilder().WithUid($"app-{i}").WithAreaId(1).Build());
        }

        var handler = CreateHandler(
            planItClient: planItClient,
            authorityProvider: authorityProvider,
            options: new PollingOptions());

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Handler continued to the natural end (page 3 is partial) rather than stopping early.
        var pages = planItClient.PagesRequested.Where(p => p.AuthorityId == 1).Select(p => p.Page).ToList();
        await Assert.That(pages).IsEquivalentTo(ExpectedPages1Through3);
    }

    [Test]
    public async Task Should_ClearCursor_When_NaturalEndReachedAfterResume()
    {
        // Seed state with an active cursor pointing at NextPage=2 — handler starts at
        // page 1 (2 - 1 overlap). Fake returns a single partial page (HasMorePages=false).
        // Cursor must be cleared and HWM advanced to the highest LastDifferent seen.
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var lastPollTime = new DateTimeOffset(2026, 4, 10, 0, 0, 0, TimeSpan.Zero);
        var pollStateStore = new FakePollStateStore();
        pollStateStore.SetState(1, new PollState(
            lastPollTime,
            new PollCursor(DateOnly.FromDateTime(lastPollTime.UtcDateTime), NextPage: 2, KnownTotal: 150)));

        var appLastDifferent = new DateTimeOffset(2026, 4, 15, 8, 0, 0, TimeSpan.Zero);

        // Single app → page 1 is partial → HasMorePages=false.
        var planItClient = new FakePlanItClient();
        planItClient.Add(1, new PlanningApplicationBuilder()
            .WithUid("app-r").WithAreaId(1).WithLastDifferent(appLastDifferent).Build());

        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        await Assert.That(pollStateStore.GetCursorFor(1)).IsNull();
        await Assert.That(pollStateStore.GetLastPollTimeFor(1)).IsEqualTo(appLastDifferent);
    }

    [Test]
    public async Task Should_IgnoreStaleCursor_When_DifferentStartDateHasAdvanced()
    {
        // Cursor recorded against 2026-04-18 but HWM has advanced to 2026-04-19.
        // Handler must treat cursor as stale and start at page 1.
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var lastPollTime = new DateTimeOffset(2026, 4, 19, 8, 0, 0, TimeSpan.Zero);
        var staleCursor = new PollCursor(
            DifferentStart: new DateOnly(2026, 4, 18),
            NextPage: 4,
            KnownTotal: 350);
        var pollStateStore = new FakePollStateStore();
        pollStateStore.SetState(1, new PollState(lastPollTime, staleCursor));

        var planItClient = new FakePlanItClient();

        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        var pages = planItClient.PagesRequested.Where(p => p.AuthorityId == 1).Select(p => p.Page).ToList();
        await Assert.That(pages).Contains(1);
    }

    [Test]
    public async Task Should_ResumeAtCursorPage_When_CursorMatchesDate()
    {
        // Cursor says NextPage=4, lastPollTime date matches — handler must start at page 3
        // (4 - 1 overlap for page-shift safety).
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var lastPollTime = new DateTimeOffset(2026, 4, 10, 0, 0, 0, TimeSpan.Zero);
        var pollStateStore = new FakePollStateStore();
        pollStateStore.SetState(1, new PollState(
            lastPollTime,
            new PollCursor(DateOnly.FromDateTime(lastPollTime.UtcDateTime), NextPage: 4, KnownTotal: 350)));

        var planItClient = new FakePlanItClient();

        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        var pages = planItClient.PagesRequested.Where(p => p.AuthorityId == 1).Select(p => p.Page).ToList();
        await Assert.That(pages).Contains(3);
        await Assert.That(pages).DoesNotContain(1);
        await Assert.That(pages).DoesNotContain(2);
    }

    [Test]
    public async Task Should_SaveCursor_When_RateLimitHitsMidPagination()
    {
        // 429 thrown AFTER page 1 yields 2 apps (page 1 had HasMorePages=true). The handler
        // then calls page 2 which throws. We expect: HWM frozen at existing lastPollTime,
        // cursor saved with NextPage=2 (the page that failed).
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var existingLastPollTime = new DateTimeOffset(2026, 4, 10, 0, 0, 0, TimeSpan.Zero);
        var pollStateStore = new FakePollStateStore();
        pollStateStore.SetLastPollTime(100, existingLastPollTime);

        // Seed three apps so the fake has something to return on page 1.
        var planItClient = new FakePlanItClient();
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(100).Build());
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-2").WithAreaId(100).Build());
        planItClient.Add(100, new PlanningApplicationBuilder().WithUid("app-3").WithAreaId(100).Build());
        planItClient.ThrowAfterYielding(
            100,
            2,
            new HttpRequestException("Rate limited", null, HttpStatusCode.TooManyRequests));

        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // HWM frozen.
        await Assert.That(pollStateStore.GetLastPollTimeFor(100)).IsEqualTo(existingLastPollTime);

        // Cursor saved at the page that failed (page 2).
        var cursor = pollStateStore.GetCursorFor(100);
        await Assert.That(cursor).IsNotNull();
        await Assert.That(cursor!.NextPage).IsEqualTo(2);
        await Assert.That(cursor.DifferentStart).IsEqualTo(DateOnly.FromDateTime(existingLastPollTime.UtcDateTime));
    }

    [Test]
    public async Task Should_StartAtPage1_When_NoCursorExists()
    {
        // No prior poll state → no cursor → handler must begin pagination at page 1.
        // Regression guard: the cursor resume branch must only fire when GetAsync
        // returns a state with a non-null Cursor.
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var planItClient = new FakePlanItClient();
        planItClient.Add(1, new PlanningApplicationBuilder().WithUid("app-1").WithAreaId(1).Build());

        var pollStateStore = new FakePollStateStore();

        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        var pages = planItClient.PagesRequested.Where(p => p.AuthorityId == 1).Select(p => p.Page).ToList();
        await Assert.That(pages).HasCount().EqualTo(1);
        await Assert.That(pages[0]).IsEqualTo(1);
    }

    [Test]
    public async Task Should_ResumeSpike_AcrossMultipleCycles()
    {
        // 7-page spike with cap=3 — drive the handler across three cycles against the
        // same seeded state:
        //   * Cycle A starts at page 1, caps out at page 3 → cursor NextPage=4, HWM frozen.
        //   * Cycle B resumes (overlap -1) from page 3, caps out at page 5 → cursor NextPage=6, HWM frozen.
        //   * Cycle C resumes (overlap -1) from page 5, reaches the natural end at page 7
        //     → cursor cleared, HWM advanced to the max LastDifferent seen.
        // All three cycles share the same FakePlanItClient and FakePollStateStore so the
        // stored cursor is what actually drives the next cycle's start page.
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var initialLastPollTime = new DateTimeOffset(2026, 4, 10, 0, 0, 0, TimeSpan.Zero);
        var pollStateStore = new FakePollStateStore();
        pollStateStore.SetLastPollTime(1, initialLastPollTime);

        // Seed 7 pages total: 6 full pages + 1 partial page so HasMorePages flips false on page 7.
        var highestLastDifferent = new DateTimeOffset(2026, 4, 15, 23, 0, 0, TimeSpan.Zero);
        var planItClient = new FakePlanItClient();
        for (var i = 0; i < (FakePlanItClient.PageSize * 6) + 10; i++)
        {
            // Make the final app on page 7 the one with the highest LastDifferent so we can
            // verify the HWM advance after natural end.
            var lastDifferent = i == (FakePlanItClient.PageSize * 6) + 9
                ? highestLastDifferent
                : initialLastPollTime.AddHours(1);
            planItClient.Add(1, new PlanningApplicationBuilder()
                .WithUid($"app-{i}")
                .WithAreaId(1)
                .WithLastDifferent(lastDifferent)
                .Build());
        }

        var options = new PollingOptions { MaxPagesPerAuthorityPerCycle = 3 };

        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider,
            options: options);

        // Cycle A
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);
        var cursorAfterA = pollStateStore.GetCursorFor(1);
        await Assert.That(cursorAfterA).IsNotNull();
        await Assert.That(cursorAfterA!.NextPage).IsEqualTo(4);
        await Assert.That(pollStateStore.GetLastPollTimeFor(1)).IsEqualTo(initialLastPollTime);

        // Cycle B — resumes at page 3 (4 - 1 overlap), processes pages 3, 4, 5 → cap hits
        // after 3 pages, so next unfetched page = 6.
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);
        var cursorAfterB = pollStateStore.GetCursorFor(1);
        await Assert.That(cursorAfterB).IsNotNull();
        await Assert.That(cursorAfterB!.NextPage).IsEqualTo(6);
        await Assert.That(pollStateStore.GetLastPollTimeFor(1)).IsEqualTo(initialLastPollTime);

        // Cycle C — resumes at page 5, processes pages 5, 6, 7. Page 7 is partial so
        // HasMorePages=false → natural end → cursor cleared and HWM advanced.
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);
        await Assert.That(pollStateStore.GetCursorFor(1)).IsNull();
        await Assert.That(pollStateStore.GetLastPollTimeFor(1)).IsEqualTo(highestLastDifferent);

        // Sanity: verify the pages actually fetched across all three cycles.
        // A: 1,2,3. B: 3,4,5. C: 5,6,7.
        var pages = planItClient.PagesRequested.Where(p => p.AuthorityId == 1).Select(p => p.Page).ToList();
        await Assert.That(pages).IsEquivalentTo(ExpectedPagesAcrossThreeCycles);
    }

    [Test]
    public async Task Should_SaveCursor_When_PageCapHits()
    {
        // Seed 5 full pages of apps for a single authority, with MaxPages=3.
        // Handler must stop after page 3 and persist a cursor pointing at page 4
        // with the high-water mark frozen at the existing lastPollTime.
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        var existingLastPollTime = new DateTimeOffset(2026, 4, 10, 0, 0, 0, TimeSpan.Zero);
        var pollStateStore = new FakePollStateStore();
        pollStateStore.SetLastPollTime(1, existingLastPollTime);

        var planItClient = new FakePlanItClient();
        for (var i = 0; i < FakePlanItClient.PageSize * 5; i++)
        {
            planItClient.Add(1, new PlanningApplicationBuilder().WithUid($"app-{i}").WithAreaId(1).Build());
        }

        var options = new PollingOptions { MaxPagesPerAuthorityPerCycle = 3 };

        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider,
            options: options);

        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // HWM frozen — lastPollTime unchanged.
        await Assert.That(pollStateStore.GetLastPollTimeFor(1)).IsEqualTo(existingLastPollTime);

        // Cursor saved pointing at page 4 (the next unfetched page).
        var cursor = pollStateStore.GetCursorFor(1);
        await Assert.That(cursor).IsNotNull();
        await Assert.That(cursor!.NextPage).IsEqualTo(4);
        await Assert.That(cursor.DifferentStart).IsEqualTo(DateOnly.FromDateTime(existingLastPollTime.UtcDateTime));
    }

    private static PollPlanItCommandHandler CreateHandler(
        FakePlanItClient? planItClient = null,
        FakePollStateStore? pollStateStore = null,
        FakePlanningApplicationRepository? repository = null,
        FakeActiveAuthorityProvider? authorityProvider = null,
        FakeWatchZoneRepository? watchZoneRepository = null,
        FakeNotificationEnqueuer? notificationEnqueuer = null,
        TimeProvider? timeProvider = null,
        ICycleSelector? cycleSelector = null,
        PollingOptions? options = null)
    {
        return new PollPlanItCommandHandler(
            planItClient ?? new FakePlanItClient(),
            pollStateStore ?? new FakePollStateStore(),
            repository ?? new FakePlanningApplicationRepository(),
            timeProvider ?? TimeProvider.System,
            authorityProvider ?? new FakeActiveAuthorityProvider(),
            watchZoneRepository ?? new FakeWatchZoneRepository(),
            notificationEnqueuer ?? new FakeNotificationEnqueuer(),
            cycleSelector ?? new FakeCycleSelector(CycleType.Watched),
            options ?? new PollingOptions(),
            NullLogger<PollPlanItCommandHandler>.Instance);
    }
}
