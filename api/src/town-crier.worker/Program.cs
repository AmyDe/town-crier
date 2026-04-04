using System.Diagnostics;
using Azure.Monitor.OpenTelemetry.Exporter;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;
using OpenTelemetry;
using OpenTelemetry.Metrics;
using OpenTelemetry.Trace;
using TownCrier.Application.Observability;
using TownCrier.Application.PlanIt;
using TownCrier.Application.PlanningApplications;
using TownCrier.Application.Polling;
using TownCrier.Application.WatchZones;
using TownCrier.Infrastructure.Cosmos;
using TownCrier.Infrastructure.Observability;
using TownCrier.Infrastructure.PlanIt;
using TownCrier.Infrastructure.PlanningApplications;
using TownCrier.Infrastructure.Polling;
using TownCrier.Infrastructure.WatchZones;
using TownCrier.Worker;

var builder = Host.CreateApplicationBuilder(args);

var hasAppInsights = !string.IsNullOrEmpty(
    builder.Configuration["APPLICATIONINSIGHTS_CONNECTION_STRING"]);

builder.Services.AddOpenTelemetry()
    .WithTracing(tracing =>
    {
        tracing
            .AddHttpClientInstrumentation()
            .AddSource(PollingInstrumentation.ActivitySourceName)
            .AddSource(CosmosInstrumentation.ActivitySourceName);

        if (hasAppInsights)
        {
            tracing.AddAzureMonitorTraceExporter();
        }
    })
    .WithMetrics(metrics =>
    {
        metrics
            .AddHttpClientInstrumentation()
            .AddMeter(PollingMetrics.MeterName)
            .AddMeter(CosmosInstrumentation.MeterName);

        if (hasAppInsights)
        {
            metrics.AddAzureMonitorMetricExporter();
        }
    });

builder.Services.AddCosmosRestClient(builder.Configuration);

builder.Services.AddSingleton<IPlanningApplicationRepository, CosmosPlanningApplicationRepository>();
builder.Services.AddSingleton<IWatchZoneRepository, CosmosWatchZoneRepository>();
builder.Services.AddSingleton<IPollStateStore, CosmosPollStateStore>();
builder.Services.AddSingleton<IActiveAuthorityProvider, WatchZoneActiveAuthorityProvider>();
builder.Services.AddSingleton<INotificationEnqueuer, LogNotificationEnqueuer>();
builder.Services.AddSingleton(TimeProvider.System);

#pragma warning disable S1075 // Hardcoded URI is a sensible default
var planItBaseUrl = builder.Configuration["PlanIt:BaseUrl"] ?? "https://www.planit.org.uk/";
#pragma warning restore S1075
builder.Services.AddHttpClient<IPlanItClient, PlanItClient>(client =>
{
    client.BaseAddress = new Uri(planItBaseUrl);
});

builder.Services.AddTransient<PollPlanItCommandHandler>();

using var host = builder.Build();

var handler = host.Services.GetRequiredService<PollPlanItCommandHandler>();
var logger = host.Services.GetRequiredService<ILoggerFactory>().CreateLogger("TownCrier.Worker");

try
{
    using var activity = PollingInstrumentation.Source.StartActivity("Polling Cycle");
    var cycleStart = Stopwatch.GetTimestamp();

    WorkerLog.PollCycleStarting(logger);

    var result = await handler.HandleAsync(new PollPlanItCommand(), CancellationToken.None)
        .ConfigureAwait(false);

    PollingMetrics.CycleDuration.Record(Stopwatch.GetElapsedTime(cycleStart).TotalMilliseconds);

    activity?.SetTag("polling.authorities_polled", result.AuthoritiesPolled);
    activity?.SetTag("polling.applications_ingested", result.ApplicationCount);

    WorkerLog.PollCycleCompleted(logger, result.ApplicationCount, result.AuthoritiesPolled);

    return 0;
}
#pragma warning disable CA1031 // Worker must return exit code on any failure
catch (Exception ex)
#pragma warning restore CA1031
{
    PollingMetrics.PollFailures.Add(1);
    WorkerLog.PollCycleFailed(logger, ex);
    return 1;
}
