using System.Diagnostics;
using System.Net;
using Microsoft.Extensions.Logging.Abstractions;
using TownCrier.Application.Observability;
using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

[NotInParallel]
public sealed class PollPlanItCommandHandlerTracingTests : IDisposable
{
    private readonly ActivityListener listener;
    private readonly List<Activity> stoppedActivities = [];

    public PollPlanItCommandHandlerTracingTests()
    {
        this.listener = new ActivityListener
        {
            ShouldListenTo = source => source.Name == PollingInstrumentation.ActivitySourceName,
            Sample = (ref ActivityCreationOptions<ActivityContext> _) => ActivitySamplingResult.AllDataAndRecorded,
            ActivityStopped = activity => this.stoppedActivities.Add(activity),
        };
        ActivitySource.AddActivityListener(this.listener);
    }

    public void Dispose()
    {
        this.listener.Dispose();
    }

    [Test]
    public async Task Should_RecordExceptionOnActivity_When_AuthorityPollFails()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var planItClient = new FakePlanItClient();
        planItClient.ThrowForAuthority(100, new HttpRequestException("Connection refused"));

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        // Clear any activities leaked from earlier tests that share the static ActivitySource
        this.stoppedActivities.Clear();

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert -- the "Poll Authority" activity should have an exception event
        var authorityActivity = this.stoppedActivities.Find(a => a.DisplayName == "Poll Authority");
        await Assert.That(authorityActivity).IsNotNull()
            .Because("an activity must be created for each authority poll");

        var exceptionEvent = authorityActivity!.Events.FirstOrDefault(e => e.Name == "exception");
        await Assert.That(exceptionEvent.Name).IsEqualTo("exception")
            .Because("the exception must be recorded on the activity for OTel export");

        var exceptionType = exceptionEvent.Tags.FirstOrDefault(t => t.Key == "exception.type").Value as string;
        await Assert.That(exceptionType).IsEqualTo("System.Net.Http.HttpRequestException");
    }

    [Test]
    public async Task Should_SetErrorStatusOnActivity_When_AuthorityPollFails()
    {
        // Arrange
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var planItClient = new FakePlanItClient();
        planItClient.ThrowForAuthority(100, new InvalidOperationException("Unexpected JSON"));

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        // Clear any activities leaked from earlier tests
        this.stoppedActivities.Clear();

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        var authorityActivity = this.stoppedActivities.Find(a => a.DisplayName == "Poll Authority");
        await Assert.That(authorityActivity).IsNotNull();
        await Assert.That(authorityActivity!.Status).IsEqualTo(ActivityStatusCode.Error)
            .Because("failed authority polls must set error status for OTel exception pipeline");
        await Assert.That(authorityActivity.StatusDescription).IsEqualTo("Unexpected JSON");
    }

    [Test]
    public async Task Should_NotRecordExceptionOnActivity_When_RateLimitHit()
    {
        // Arrange -- 429 is an expected, handled outcome (see bd tc-qc65).
        // The handler skips the authority, increments rate_limited, and saves
        // a resumable cursor (via tc-6l54). Emitting an exception event on the
        // span mislabels this as an error in App Insights, so do not record it.
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(100);

        var planItClient = new FakePlanItClient();
        planItClient.ThrowForAuthority(100, new HttpRequestException("Rate limited", null, HttpStatusCode.TooManyRequests));

        var handler = CreateHandler(planItClient: planItClient, authorityProvider: authorityProvider);

        // Clear any activities leaked from earlier tests
        this.stoppedActivities.Clear();

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert -- rate limit must NOT produce an exception event or Error status
        var authorityActivity = this.stoppedActivities.Find(a => a.DisplayName == "Poll Authority");
        await Assert.That(authorityActivity).IsNotNull();

        var exceptionEvent = authorityActivity!.Events.FirstOrDefault(e => e.Name == "exception");
        await Assert.That(exceptionEvent.Name).IsNull()
            .Because("429 is an expected, handled outcome — it must not appear as an exception in App Insights");

        await Assert.That(authorityActivity.Status).IsNotEqualTo(ActivityStatusCode.Error)
            .Because("429 is an expected, handled outcome — the authority span status must not be Error");
    }

    private static PollPlanItCommandHandler CreateHandler(
        FakePlanItClient? planItClient = null,
        FakePollStateStore? pollStateStore = null,
        FakePlanningApplicationRepository? repository = null,
        FakeActiveAuthorityProvider? authorityProvider = null,
        FakeWatchZoneRepository? watchZoneRepository = null,
        FakeNotificationEnqueuer? notificationEnqueuer = null,
        FakeDecisionEventDispatcher? decisionEventDispatcher = null,
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
            decisionEventDispatcher ?? new FakeDecisionEventDispatcher(),
            cycleSelector ?? new FakeCycleSelector(CycleType.Watched),
            options ?? new PollingOptions(),
            NullLogger<PollPlanItCommandHandler>.Instance);
    }
}
