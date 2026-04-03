using System.Diagnostics;
using TownCrier.Application.Observability;
using TownCrier.Application.Polling;

namespace TownCrier.Web.Polling;

internal sealed partial class PlanItPollingService : BackgroundService
{
    private readonly IServiceScopeFactory scopeFactory;
    private readonly TimeSpan interval;
    private readonly ILogger<PlanItPollingService> logger;
    private int cycleNumber;

    public PlanItPollingService(
        IServiceScopeFactory scopeFactory,
        IConfiguration configuration,
        ILogger<PlanItPollingService> logger)
    {
        this.scopeFactory = scopeFactory;
        this.logger = logger;

        var minutes = configuration.GetValue("Polling:IntervalMinutes", 15);
        this.interval = TimeSpan.FromMinutes(minutes);
    }

    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        LogPollingStarted(this.logger, this.interval.TotalMinutes);

        using var timer = new PeriodicTimer(this.interval);

        while (await timer.WaitForNextTickAsync(stoppingToken).ConfigureAwait(false))
        {
            try
            {
                using var activity = PollingInstrumentation.Source.StartActivity("Polling Cycle");
                activity?.SetTag("polling.cycle_number", this.cycleNumber);
                var cycleStart = Stopwatch.GetTimestamp();

                var scope = this.scopeFactory.CreateAsyncScope();
                await using (scope.ConfigureAwait(false))
                {
                    var handler = scope.ServiceProvider.GetRequiredService<PollPlanItCommandHandler>();

                    var result = await handler.HandleAsync(new PollPlanItCommand(this.cycleNumber), stoppingToken).ConfigureAwait(false);

                    activity?.SetTag("polling.authorities_total", result.TotalActiveAuthorities);
                    activity?.SetTag("polling.authorities_polled", result.AuthoritiesPolled);
                    activity?.SetTag("polling.authorities_skipped", result.AuthoritiesSkipped);
                    activity?.SetTag("polling.applications_ingested", result.ApplicationCount);

                    PollingMetrics.CycleDuration.Record(Stopwatch.GetElapsedTime(cycleStart).TotalMilliseconds);

                    LogPollCycleCompleted(this.logger, result.ApplicationCount, this.cycleNumber, result.AuthoritiesPolled, result.AuthoritiesSkipped);
                }

                this.cycleNumber++;
            }
            catch (OperationCanceledException) when (stoppingToken.IsCancellationRequested)
            {
                break;
            }
#pragma warning disable CA1031 // Polling loop must not crash on transient errors
            catch (Exception ex)
#pragma warning restore CA1031
            {
                LogPollCycleFailed(this.logger, ex);
                this.cycleNumber++;
            }
        }

        LogPollingStopped(this.logger);
    }

    [LoggerMessage(Level = LogLevel.Information, Message = "PlanIt polling started with interval {IntervalMinutes}m")]
    private static partial void LogPollingStarted(ILogger logger, double intervalMinutes);

    [LoggerMessage(Level = LogLevel.Information, Message = "Poll cycle {CycleNumber} completed: {ApplicationCount} applications, {AuthoritiesPolled} polled, {AuthoritiesSkipped} skipped")]
    private static partial void LogPollCycleCompleted(ILogger logger, int applicationCount, int cycleNumber, int authoritiesPolled, int authoritiesSkipped);

    [LoggerMessage(Level = LogLevel.Error, Message = "Poll cycle failed")]
    private static partial void LogPollCycleFailed(ILogger logger, Exception exception);

    [LoggerMessage(Level = LogLevel.Information, Message = "PlanIt polling stopped")]
    private static partial void LogPollingStopped(ILogger logger);
}
