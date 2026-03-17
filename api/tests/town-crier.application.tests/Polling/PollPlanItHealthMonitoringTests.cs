using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

public sealed class PollPlanItHealthMonitoringTests
{
    private static readonly DateTimeOffset FixedTime = new(2026, 3, 16, 12, 0, 0, TimeSpan.Zero);

    private static readonly PollingHealthConfig DefaultConfig = new(
        StalenessThreshold: TimeSpan.FromHours(2),
        MaxConsecutiveFailures: 3);

    [Test]
    public async Task Should_RecordSuccessfulPollTime_When_PollSucceeds()
    {
        // Arrange
        var healthStore = new FakePollingHealthStore();
        var handler = CreateHandler(healthStore: healthStore);

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        var health = await healthStore.GetAsync(CancellationToken.None);
        await Assert.That(health.LastSuccessfulPollTime).IsEqualTo(FixedTime);
    }

    [Test]
    public async Task Should_ResetConsecutiveFailures_When_PollSucceeds()
    {
        // Arrange
        var healthStore = new FakePollingHealthStore();
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        // First, cause some failures
        var failingClient = new FakePlanItClient { ExceptionToThrow = new HttpRequestException("timeout") };
        var failHandler = CreateHandler(
            planItClient: failingClient,
            healthStore: healthStore,
            authorityProvider: authorityProvider);
        await SwallowAsync<HttpRequestException>(
            () => failHandler.HandleAsync(new PollPlanItCommand(), CancellationToken.None));
        await SwallowAsync<HttpRequestException>(
            () => failHandler.HandleAsync(new PollPlanItCommand(), CancellationToken.None));

        // Now succeed
        var successClient = new FakePlanItClient();
        var successHandler = CreateHandler(
            planItClient: successClient,
            healthStore: healthStore,
            authorityProvider: authorityProvider);

        // Act
        await successHandler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        var health = await healthStore.GetAsync(CancellationToken.None);
        await Assert.That(health.ConsecutiveFailureCount).IsEqualTo(0);
    }

    [Test]
    public async Task Should_IncrementConsecutiveFailures_When_PollThrows()
    {
        // Arrange
        var healthStore = new FakePollingHealthStore();
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);
        var failingClient = new FakePlanItClient { ExceptionToThrow = new HttpRequestException("timeout") };
        var handler = CreateHandler(
            planItClient: failingClient,
            healthStore: healthStore,
            authorityProvider: authorityProvider);

        // Act
        await SwallowAsync<HttpRequestException>(
            () => handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None));

        // Assert
        var health = await healthStore.GetAsync(CancellationToken.None);
        await Assert.That(health.ConsecutiveFailureCount).IsEqualTo(1);
    }

    [Test]
    public async Task Should_IncrementConsecutiveFailuresMultipleTimes_When_PollFailsRepeatedly()
    {
        // Arrange
        var healthStore = new FakePollingHealthStore();
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);
        var failingClient = new FakePlanItClient { ExceptionToThrow = new HttpRequestException("timeout") };
        var handler = CreateHandler(
            planItClient: failingClient,
            healthStore: healthStore,
            authorityProvider: authorityProvider);

        // Act
        await SwallowAsync<HttpRequestException>(
            () => handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None));
        await SwallowAsync<HttpRequestException>(
            () => handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None));
        await SwallowAsync<HttpRequestException>(
            () => handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None));

        // Assert
        var health = await healthStore.GetAsync(CancellationToken.None);
        await Assert.That(health.ConsecutiveFailureCount).IsEqualTo(3);
    }

    [Test]
    public async Task Should_AlertConsecutiveFailures_When_ThresholdExceeded()
    {
        // Arrange
        var healthStore = new FakePollingHealthStore();
        var alerter = new SpyPollingHealthAlerter();
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);
        var failingClient = new FakePlanItClient { ExceptionToThrow = new HttpRequestException("timeout") };
        var handler = CreateHandler(
            planItClient: failingClient,
            healthStore: healthStore,
            alerter: alerter,
            authorityProvider: authorityProvider,
            config: new PollingHealthConfig(TimeSpan.FromHours(2), MaxConsecutiveFailures: 3));

        // Act — fail 3 times (threshold is 3)
        await SwallowAsync<HttpRequestException>(
            () => handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None));
        await SwallowAsync<HttpRequestException>(
            () => handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None));
        await SwallowAsync<HttpRequestException>(
            () => handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None));

        // Assert
        await Assert.That(alerter.ConsecutiveFailureAlerts).HasCount().EqualTo(1);
        await Assert.That(alerter.ConsecutiveFailureAlerts[0]).IsEqualTo(3);
    }

    [Test]
    public async Task Should_NotAlertConsecutiveFailures_When_BelowThreshold()
    {
        // Arrange
        var healthStore = new FakePollingHealthStore();
        var alerter = new SpyPollingHealthAlerter();
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);
        var failingClient = new FakePlanItClient { ExceptionToThrow = new HttpRequestException("timeout") };
        var handler = CreateHandler(
            planItClient: failingClient,
            healthStore: healthStore,
            alerter: alerter,
            authorityProvider: authorityProvider,
            config: new PollingHealthConfig(TimeSpan.FromHours(2), MaxConsecutiveFailures: 3));

        // Act — fail only twice (threshold is 3)
        await SwallowAsync<HttpRequestException>(
            () => handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None));
        await SwallowAsync<HttpRequestException>(
            () => handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None));

        // Assert
        await Assert.That(alerter.ConsecutiveFailureAlerts).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_AlertStaleness_When_DataExceedsStalenessThreshold()
    {
        // Arrange
        var healthStore = new FakePollingHealthStore();
        var alerter = new SpyPollingHealthAlerter();
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);

        // Simulate a previous successful poll 3 hours ago
        var threeHoursAgo = FixedTime.AddHours(-3);
        var health = await healthStore.GetAsync(CancellationToken.None);
        health.RecordSuccess(threeHoursAgo);
        await healthStore.SaveAsync(health, CancellationToken.None);

        // Now the poll fails
        var failingClient = new FakePlanItClient { ExceptionToThrow = new HttpRequestException("timeout") };
        var handler = CreateHandler(
            planItClient: failingClient,
            healthStore: healthStore,
            alerter: alerter,
            authorityProvider: authorityProvider,
            config: new PollingHealthConfig(StalenessThreshold: TimeSpan.FromHours(2), MaxConsecutiveFailures: 5));

        // Act
        await SwallowAsync<HttpRequestException>(
            () => handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None));

        // Assert
        await Assert.That(alerter.StalenessAlerts).HasCount().EqualTo(1);
        await Assert.That(alerter.StalenessAlerts[0].LastSuccessfulPoll).IsEqualTo(threeHoursAgo);
    }

    [Test]
    public async Task Should_NotAlertStaleness_When_PollSucceeds()
    {
        // Arrange
        var healthStore = new FakePollingHealthStore();
        var alerter = new SpyPollingHealthAlerter();
        var handler = CreateHandler(healthStore: healthStore, alerter: alerter);

        // Act
        await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None);

        // Assert
        await Assert.That(alerter.StalenessAlerts).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_NotSaveLastPollTime_When_PollFails()
    {
        // Arrange
        var pollStateStore = new FakePollStateStore();
        var healthStore = new FakePollingHealthStore();
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);
        var failingClient = new FakePlanItClient { ExceptionToThrow = new HttpRequestException("timeout") };
        var handler = CreateHandler(
            planItClient: failingClient,
            pollStateStore: pollStateStore,
            healthStore: healthStore,
            authorityProvider: authorityProvider);

        // Act
        await SwallowAsync<HttpRequestException>(
            () => handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None));

        // Assert — poll state should NOT be updated on failure
        await Assert.That(pollStateStore.LastPollTime).IsNull();
    }

    [Test]
    public async Task Should_RethrowException_When_PollFails()
    {
        // Arrange
        var healthStore = new FakePollingHealthStore();
        var authorityProvider = new FakeActiveAuthorityProvider();
        authorityProvider.Add(1);
        var failingClient = new FakePlanItClient { ExceptionToThrow = new HttpRequestException("timeout") };
        var handler = CreateHandler(
            planItClient: failingClient,
            healthStore: healthStore,
            authorityProvider: authorityProvider);

        // Act & Assert
        await Assert.That(
            async () => await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None))
            .ThrowsException()
            .OfType<HttpRequestException>();
    }

    private static async Task SwallowAsync<TException>(Func<Task> action)
        where TException : Exception
    {
        try
        {
            await action().ConfigureAwait(false);
        }
        catch (TException)
        {
            // Expected — swallowed intentionally for test setup
        }
    }

    private static PollPlanItCommandHandler CreateHandler(
        FakePlanItClient? planItClient = null,
        FakePollStateStore? pollStateStore = null,
        FakePlanningApplicationRepository? repository = null,
        FakePollingHealthStore? healthStore = null,
        SpyPollingHealthAlerter? alerter = null,
        FakeActiveAuthorityProvider? authorityProvider = null,
        PollingHealthConfig? config = null)
    {
        return new PollPlanItCommandHandler(
            planItClient ?? new FakePlanItClient(),
            pollStateStore ?? new FakePollStateStore(),
            repository ?? new FakePlanningApplicationRepository(),
            new FakeTimeProvider(FixedTime),
            authorityProvider ?? new FakeActiveAuthorityProvider(),
            healthStore ?? new FakePollingHealthStore(),
            alerter ?? new SpyPollingHealthAlerter(),
            config ?? DefaultConfig,
            new FakeWatchZoneRepository(),
            new FakeNotificationEnqueuer());
    }
}
