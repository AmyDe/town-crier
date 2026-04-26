using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

/// <summary>
/// Regression coverage for tc-ews7 — Seed-cycle starvation. Locks down the
/// invariant that authority selection inside the Seed cycle depends only on
/// <see cref="PollState.LastPollTime"/>, never on <see cref="PollState.HighWaterMark"/>.
///
/// Background: PR #298 (tc-m6fx) split <c>LastPollTime</c> from <c>HighWaterMark</c>
/// so quiet authorities don't repeatedly sort to the front of the queue. After
/// that deploy, 272 hwm-empty authorities went 12.5h+ without being polled while
/// 213 hwm-set authorities polled normally — the empirical fairness signature of
/// HWM leaking back into selection. These tests assert HWM is never a selection
/// or ordering criterion, so any future regression is caught at unit level.
/// </summary>
public sealed class PollPlanItCommandHandlerSeedFairnessTests
{
    [Test]
    public async Task Should_PollAuthorityWithStaleHwmFirst_When_LastPollTimeIsOlder_DuringSeedCycle()
    {
        // Arrange — authority 100 has a fresh HWM (lots of recent apps) but
        // was just polled. Authority 200 has a stale HWM (quiet authority,
        // hwm=null in Cosmos parlance) and was last polled days ago. Selection
        // must follow LastPollTime ascending, so 200 polls first regardless
        // of HWM.
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);

        var now = new DateTimeOffset(2026, 4, 25, 12, 0, 0, TimeSpan.Zero);
        var pollStateStore = new FakePollStateStore();

        // Authority 100: HWM is fresh (1h ago), but it was polled 5 minutes ago.
        pollStateStore.SetState(100, new PollState(
            LastPollTime: now.AddMinutes(-5),
            HighWaterMark: now.AddHours(-1),
            Cursor: null));

        // Authority 200: HWM is ancient (representing hwm=null / never-ingested),
        // and it was last polled 7 days ago. By LastPollTime alone, this should
        // be polled FIRST.
        pollStateStore.SetState(200, new PollState(
            LastPollTime: now.AddDays(-7),
            HighWaterMark: DateTimeOffset.MinValue,
            Cursor: null));

        var planItClient = new FakePlanItClient();

        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider,
            cycleSelector: new FakeCycleSelector(CycleType.Seed),
            timeProvider: new FakeTimeProvider(now));

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — 200 (stale HWM, oldest LastPollTime) polls before 100.
        await Assert.That(planItClient.AuthorityIdsRequested).HasCount().EqualTo(2);
        await Assert.That(planItClient.AuthorityIdsRequested[0]).IsEqualTo(200);
        await Assert.That(planItClient.AuthorityIdsRequested[1]).IsEqualTo(100);
    }

    [Test]
    public async Task Should_PollNeverPolledAuthoritiesFirst_When_OthersPolledRecently_DuringSeedCycle()
    {
        // Arrange — never-polled authorities (no PollState document at all)
        // must always sort ahead of any polled authority, regardless of HWM
        // freshness on the polled set. This is the structural fix that drains
        // the hwm=null backlog in production.
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);
        authorityProvider.Add(200);
        authorityProvider.Add(300);

        var now = new DateTimeOffset(2026, 4, 25, 12, 0, 0, TimeSpan.Zero);
        var pollStateStore = new FakePollStateStore();

        // Authority 100 polled recently with a healthy HWM.
        pollStateStore.SetState(100, new PollState(
            LastPollTime: now.AddMinutes(-1),
            HighWaterMark: now.AddMinutes(-30),
            Cursor: null));

        // 200 and 300 have NO PollState document — they have never been polled.
        // They must be polled first.
        var planItClient = new FakePlanItClient();

        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider,
            cycleSelector: new FakeCycleSelector(CycleType.Seed),
            timeProvider: new FakeTimeProvider(now));

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — 200 and 300 (never polled) come before 100.
        await Assert.That(planItClient.AuthorityIdsRequested).HasCount().EqualTo(3);
        await Assert.That(planItClient.AuthorityIdsRequested[0]).IsNotEqualTo(100);
        await Assert.That(planItClient.AuthorityIdsRequested[1]).IsNotEqualTo(100);
        await Assert.That(planItClient.AuthorityIdsRequested[2]).IsEqualTo(100);
    }

    [Test]
    public async Task Should_BumpLastPollTime_When_PollReturnsZeroApplications()
    {
        // Arrange — a quiet authority that returns zero apps must still have
        // its LastPollTime advanced to `now` so it rotates off the top of
        // the LRU queue. Without this, never-polled / quiet authorities would
        // sort first on every cycle and starve the rest.
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(500);

        var now = new DateTimeOffset(2026, 4, 25, 12, 0, 0, TimeSpan.Zero);
        var pollStateStore = new FakePollStateStore();

        // Pre-existing stale state — last polled 5 days ago.
        pollStateStore.SetState(500, new PollState(
            LastPollTime: now.AddDays(-5),
            HighWaterMark: now.AddDays(-30),
            Cursor: null));

        // FakePlanItClient returns zero applications by default — exactly the
        // "quiet authority" case.
        var planItClient = new FakePlanItClient();

        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider,
            cycleSelector: new FakeCycleSelector(CycleType.Seed),
            timeProvider: new FakeTimeProvider(now));

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — LastPollTime advanced to `now`, even though no apps returned.
        var savedLastPollTime = pollStateStore.GetLastPollTimeFor(500);
        await Assert.That(savedLastPollTime).IsEqualTo(now);
    }

    [Test]
    public async Task Should_BumpLastPollTime_When_FirstEverPollReturnsZeroApplications()
    {
        // Arrange — a never-polled authority that returns zero apps on its
        // first poll must still get a PollState document written so it
        // rotates off the front of the LRU. This is the "first poll for a
        // hwm=null authority" path that drains the 272-authority backlog
        // observed in production.
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(700);

        var now = new DateTimeOffset(2026, 4, 25, 12, 0, 0, TimeSpan.Zero);

        // No pre-existing state for 700 in this fake store.
        var pollStateStore = new FakePollStateStore();
        var planItClient = new FakePlanItClient();

        var handler = CreateHandler(
            planItClient: planItClient,
            pollStateStore: pollStateStore,
            authorityProvider: authorityProvider,
            cycleSelector: new FakeCycleSelector(CycleType.Seed),
            timeProvider: new FakeTimeProvider(now));

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — a state document was written with LastPollTime = now.
        var savedLastPollTime = pollStateStore.GetLastPollTimeFor(700);
        await Assert.That(savedLastPollTime).IsNotNull();
        await Assert.That(savedLastPollTime!.Value).IsEqualTo(now);
    }

    [Test]
    public async Task Should_PollOnlyWatchZoneAuthorities_When_CycleIsWatched()
    {
        // Arrange — Watched cycle behavior must remain unchanged: only watch-zone
        // authorities are polled, the wider all-authority pool is ignored. This
        // preserves prioritization for paying users while the Seed cycle handles
        // the rest of the pool.
        var watchZoneProvider = new FakeWatchZoneActiveAuthorityProvider();
        watchZoneProvider.Add(11);
        watchZoneProvider.Add(22);

        var allProvider = new FakeAllAuthorityIdProvider();
        allProvider.Add(11);
        allProvider.Add(22);
        allProvider.Add(33); // not in watch-zone set
        allProvider.Add(44); // not in watch-zone set

        var cycleSelector = new FakeCycleSelector(CycleType.Watched);
        var alternating = new CycleAlternatingAuthorityProvider(
            watchZoneProvider, allProvider, cycleSelector);

        var planItClient = new FakePlanItClient();

        var now = new DateTimeOffset(2026, 4, 25, 12, 0, 0, TimeSpan.Zero);
        var handler = new PollPlanItCommandHandler(
            planItClient,
            new FakePollStateStore(),
            new FakePlanningApplicationRepository(),
            new FakeTimeProvider(now),
            alternating,
            new FakeWatchZoneRepository(),
            new FakeNotificationEnqueuer(),
            new FakeDecisionAlertDispatcher(),
            cycleSelector,
            new PollingOptions(),
            NullLogger<PollPlanItCommandHandler>.Instance);

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — only watch-zone authorities polled; 33 and 44 are excluded.
        await Assert.That(planItClient.AuthorityIdsRequested).HasCount().EqualTo(2);
        await Assert.That(planItClient.AuthorityIdsRequested).Contains(11);
        await Assert.That(planItClient.AuthorityIdsRequested).Contains(22);
        await Assert.That(planItClient.AuthorityIdsRequested).DoesNotContain(33);
        await Assert.That(planItClient.AuthorityIdsRequested).DoesNotContain(44);
    }

    [Test]
    public async Task Should_PollAllAuthorities_When_CycleIsSeed()
    {
        // Arrange — Seed cycle returns the wider all-authority pool (which
        // includes any watch-zone authorities). The Seed cycle is the
        // mechanism that keeps the never-polled / starved hwm=null authorities
        // from being orphaned by the Watched cycle's narrower scope.
        var watchZoneProvider = new FakeWatchZoneActiveAuthorityProvider();
        watchZoneProvider.Add(11);

        var allProvider = new FakeAllAuthorityIdProvider();
        allProvider.Add(11);
        allProvider.Add(33);
        allProvider.Add(44);

        var cycleSelector = new FakeCycleSelector(CycleType.Seed);
        var alternating = new CycleAlternatingAuthorityProvider(
            watchZoneProvider, allProvider, cycleSelector);

        var planItClient = new FakePlanItClient();

        var now = new DateTimeOffset(2026, 4, 25, 12, 0, 0, TimeSpan.Zero);
        var handler = new PollPlanItCommandHandler(
            planItClient,
            new FakePollStateStore(),
            new FakePlanningApplicationRepository(),
            new FakeTimeProvider(now),
            alternating,
            new FakeWatchZoneRepository(),
            new FakeNotificationEnqueuer(),
            new FakeDecisionAlertDispatcher(),
            cycleSelector,
            new PollingOptions(),
            NullLogger<PollPlanItCommandHandler>.Instance);

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert — all three authorities polled, including 33 and 44 outside
        // the watch-zone set.
        await Assert.That(planItClient.AuthorityIdsRequested).HasCount().EqualTo(3);
        await Assert.That(planItClient.AuthorityIdsRequested).Contains(11);
        await Assert.That(planItClient.AuthorityIdsRequested).Contains(33);
        await Assert.That(planItClient.AuthorityIdsRequested).Contains(44);
    }

    private static PollPlanItCommandHandler CreateHandler(
        FakePlanItClient? planItClient = null,
        FakePollStateStore? pollStateStore = null,
        FakePlanningApplicationRepository? repository = null,
        FakeActiveAuthorityProvider? authorityProvider = null,
        FakeWatchZoneRepository? watchZoneRepository = null,
        FakeNotificationEnqueuer? notificationEnqueuer = null,
        FakeDecisionAlertDispatcher? decisionAlertDispatcher = null,
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
            decisionAlertDispatcher ?? new FakeDecisionAlertDispatcher(),
            cycleSelector ?? new FakeCycleSelector(CycleType.Watched),
            options ?? new PollingOptions(),
            NullLogger<PollPlanItCommandHandler>.Instance);
    }
}
