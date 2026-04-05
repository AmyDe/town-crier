using System.Diagnostics;
using Azure.Monitor.OpenTelemetry.Exporter;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;
using OpenTelemetry;
using OpenTelemetry.Metrics;
using OpenTelemetry.Trace;
using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.Notifications;
using TownCrier.Application.Observability;
using TownCrier.Application.PlanIt;
using TownCrier.Application.PlanningApplications;
using TownCrier.Application.Polling;
using TownCrier.Application.UserProfiles;
using TownCrier.Application.WatchZones;
using TownCrier.Infrastructure.Cosmos;
using TownCrier.Infrastructure.DeviceRegistrations;
using TownCrier.Infrastructure.Notifications;
using TownCrier.Infrastructure.Observability;
using TownCrier.Infrastructure.PlanIt;
using TownCrier.Infrastructure.PlanningApplications;
using TownCrier.Infrastructure.Polling;
using TownCrier.Infrastructure.UserProfiles;
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
            .AddMeter(ApiMetrics.MeterName)
            .AddMeter(CosmosInstrumentation.MeterName)
            .AddMeter(PlanItInstrumentation.MeterName);

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
builder.Services.AddSingleton<INotificationRepository, CosmosNotificationRepository>();
builder.Services.AddSingleton<IUserProfileRepository, CosmosUserProfileRepository>();
builder.Services.AddSingleton<IDeviceRegistrationRepository, CosmosDeviceRegistrationRepository>();
builder.Services.AddSingleton<IPushNotificationSender, NoOpPushNotificationSender>();

var acsConnectionString = builder.Configuration["AzureCommunicationServices:ConnectionString"];
if (!string.IsNullOrEmpty(acsConnectionString))
{
    try
    {
        builder.Services.AddSingleton<IEmailSender>(sp =>
            new AcsEmailSender(acsConnectionString, sp.GetRequiredService<ILogger<AcsEmailSender>>()));
    }
    catch (InvalidOperationException)
    {
        builder.Services.AddSingleton<IEmailSender, NoOpEmailSender>();
    }
}
else
{
    builder.Services.AddSingleton<IEmailSender, NoOpEmailSender>();
}

builder.Services.AddSingleton<DispatchNotificationCommandHandler>();
builder.Services.AddSingleton<INotificationEnqueuer, DispatchNotificationEnqueuer>();
builder.Services.AddSingleton(TimeProvider.System);

#pragma warning disable S1075 // Hardcoded URI is a sensible default
var planItBaseUrl = builder.Configuration["PlanIt:BaseUrl"] ?? "https://www.planit.org.uk/";
#pragma warning restore S1075
builder.Services.AddHttpClient<IPlanItClient, PlanItClient>(client =>
{
    client.BaseAddress = new Uri(planItBaseUrl);
});

builder.Services.AddTransient<PollPlanItCommandHandler>();
builder.Services.AddSingleton<GenerateWeeklyDigestsCommandHandler>();

using var host = builder.Build();

// Eagerly initialize OTel providers so they listen before metrics are recorded.
// Without this, Counter.Add() / Histogram.Record() silently drop measurements
// because the providers are lazy singletons first resolved at ForceFlush time.
_ = host.Services.GetRequiredService<MeterProvider>();
_ = host.Services.GetRequiredService<TracerProvider>();

var mode = builder.Configuration["WORKER_MODE"] ?? "poll";
var logger = host.Services.GetRequiredService<ILoggerFactory>().CreateLogger("TownCrier.Worker");

var exitCode = 0;

switch (mode)
{
    case "poll":
        try
        {
            using var activity = PollingInstrumentation.Source.StartActivity("Polling Cycle");
            var cycleStart = Stopwatch.GetTimestamp();

            WorkerLog.PollCycleStarting(logger);

            var pollHandler = host.Services.GetRequiredService<PollPlanItCommandHandler>();
            var result = await pollHandler.HandleAsync(new PollPlanItCommand(), CancellationToken.None)
                .ConfigureAwait(false);

            PollingMetrics.CycleDuration.Record(Stopwatch.GetElapsedTime(cycleStart).TotalMilliseconds);

            activity?.SetTag("polling.authorities_polled", result.AuthoritiesPolled);
            activity?.SetTag("polling.applications_ingested", result.ApplicationCount);

            WorkerLog.PollCycleCompleted(logger, result.ApplicationCount, result.AuthoritiesPolled);
        }
#pragma warning disable CA1031 // Worker must return exit code on any failure
        catch (Exception ex)
#pragma warning restore CA1031
        {
            PollingMetrics.PollFailures.Add(1);
            WorkerLog.PollCycleFailed(logger, ex);
            exitCode = 1;
        }

        break;

    case "digest":
        try
        {
            WorkerLog.DigestCycleStarting(logger);

            var digestHandler = host.Services.GetRequiredService<GenerateWeeklyDigestsCommandHandler>();
            await digestHandler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None)
                .ConfigureAwait(false);

            WorkerLog.DigestCycleCompleted(logger);
        }
#pragma warning disable CA1031 // Worker must return exit code on any failure
        catch (Exception ex)
#pragma warning restore CA1031
        {
            WorkerLog.DigestCycleFailed(logger, ex);
            exitCode = 1;
        }

        break;

    default:
        WorkerLog.UnknownWorkerMode(logger, mode);
        exitCode = 1;
        break;
}

// Force-flush OpenTelemetry before the short-lived process exits.
// The Azure Monitor exporter batches on ~30 s intervals; without this,
// the worker terminates before the first batch window.
try
{
    host.Services.GetService<MeterProvider>()?.ForceFlush(timeoutMilliseconds: 10_000);
}
#pragma warning disable CA1031 // Flush failure must not mask the original error
catch (Exception)
{
    // Intentionally swallowed — flush failure must not mask the original exit code.
}
#pragma warning restore CA1031

try
{
    host.Services.GetService<TracerProvider>()?.ForceFlush(timeoutMilliseconds: 10_000);
}
#pragma warning disable CA1031 // Flush failure must not mask the original error
catch (Exception)
{
    // Intentionally swallowed — flush failure must not mask the original exit code.
}
#pragma warning restore CA1031

return exitCode;
