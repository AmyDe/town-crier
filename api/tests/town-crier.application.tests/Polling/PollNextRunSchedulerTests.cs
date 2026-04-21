using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

public sealed class PollNextRunSchedulerTests
{
    private static readonly DateTimeOffset Now = new(2026, 4, 21, 12, 0, 0, TimeSpan.Zero);
    private static readonly PollNextRunSchedulerOptions DefaultOptions = new();

    [Test]
    public async Task Should_ScheduleAtNowPlusNaturalCadence_When_TerminationIsNatural()
    {
        var jitter = new ZeroJitter();
        var scheduler = new PollNextRunScheduler(DefaultOptions, jitter);

        var next = scheduler.ComputeNextRun(PollTerminationReason.Natural, retryAfter: null, Now);

        await Assert.That(next).IsEqualTo(Now + DefaultOptions.NaturalCadence);
    }

    [Test]
    public async Task Should_ScheduleAtNowPlusTimeBoundedCadence_When_TerminationIsTimeBounded()
    {
        var jitter = new ZeroJitter();
        var scheduler = new PollNextRunScheduler(DefaultOptions, jitter);

        var next = scheduler.ComputeNextRun(PollTerminationReason.TimeBounded, retryAfter: null, Now);

        await Assert.That(next).IsEqualTo(Now + DefaultOptions.TimeBoundedCadence);
    }

    [Test]
    public async Task Should_ScheduleAtNowPlusRetryAfter_When_RateLimitedWithSmallRetryAfter()
    {
        var jitter = new ZeroJitter();
        var scheduler = new PollNextRunScheduler(DefaultOptions, jitter);
        var retryAfter = TimeSpan.FromMinutes(2);

        var next = scheduler.ComputeNextRun(PollTerminationReason.RateLimited, retryAfter, Now);

        await Assert.That(next).IsEqualTo(Now + retryAfter);
    }

    [Test]
    public async Task Should_CapRetryAfterAt30Minutes_When_RateLimitedWithLargeRetryAfter()
    {
        var jitter = new ZeroJitter();
        var scheduler = new PollNextRunScheduler(DefaultOptions, jitter);
        var retryAfter = TimeSpan.FromHours(2);

        var next = scheduler.ComputeNextRun(PollTerminationReason.RateLimited, retryAfter, Now);

        await Assert.That(next).IsEqualTo(Now + DefaultOptions.RetryAfterCap);
    }

    [Test]
    public async Task Should_FallBackToRateLimitDefault_When_RateLimitedWithoutRetryAfter()
    {
        var jitter = new ZeroJitter();
        var scheduler = new PollNextRunScheduler(DefaultOptions, jitter);

        var next = scheduler.ComputeNextRun(PollTerminationReason.RateLimited, retryAfter: null, Now);

        await Assert.That(next).IsEqualTo(Now + DefaultOptions.RateLimitDefault);
    }

    [Test]
    public async Task Should_ApplyPositiveJitter_When_RateLimited()
    {
        var jitter = new FixedJitter(TimeSpan.FromSeconds(7));
        var scheduler = new PollNextRunScheduler(DefaultOptions, jitter);
        var retryAfter = TimeSpan.FromMinutes(1);

        var next = scheduler.ComputeNextRun(PollTerminationReason.RateLimited, retryAfter, Now);

        await Assert.That(next).IsEqualTo(Now + retryAfter + TimeSpan.FromSeconds(7));
    }

    [Test]
    public async Task Should_ApplyNegativeJitter_When_RateLimited()
    {
        var jitter = new FixedJitter(TimeSpan.FromSeconds(-4));
        var scheduler = new PollNextRunScheduler(DefaultOptions, jitter);
        var retryAfter = TimeSpan.FromMinutes(1);

        var next = scheduler.ComputeNextRun(PollTerminationReason.RateLimited, retryAfter, Now);

        await Assert.That(next).IsEqualTo(Now + retryAfter + TimeSpan.FromSeconds(-4));
    }

    [Test]
    public async Task Should_NotApplyJitter_When_Natural()
    {
        var jitter = new FixedJitter(TimeSpan.FromSeconds(7));
        var scheduler = new PollNextRunScheduler(DefaultOptions, jitter);

        var next = scheduler.ComputeNextRun(PollTerminationReason.Natural, retryAfter: null, Now);

        await Assert.That(next).IsEqualTo(Now + DefaultOptions.NaturalCadence);
    }

    private sealed class ZeroJitter : IPollJitter
    {
        public TimeSpan NextOffset(TimeSpan bound) => TimeSpan.Zero;
    }

    private sealed class FixedJitter : IPollJitter
    {
        private readonly TimeSpan value;

        public FixedJitter(TimeSpan value)
        {
            this.value = value;
        }

        public TimeSpan NextOffset(TimeSpan bound) => this.value;
    }
}
