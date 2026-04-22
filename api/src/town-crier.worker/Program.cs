using System.Diagnostics;
using Azure.Monitor.OpenTelemetry.Exporter;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;
using OpenTelemetry;
using OpenTelemetry.Logs;
using OpenTelemetry.Metrics;
using OpenTelemetry.Resources;
using OpenTelemetry.Trace;
using TownCrier.Application.Auth;
using TownCrier.Application.Authorities;
using TownCrier.Application.DecisionAlerts;
using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.Notifications;
using TownCrier.Application.Observability;
using TownCrier.Application.PlanIt;
using TownCrier.Application.PlanningApplications;
using TownCrier.Application.Polling;
using TownCrier.Application.SavedApplications;
using TownCrier.Application.UserProfiles;
using TownCrier.Application.WatchZones;
using TownCrier.Infrastructure.Auth;
using TownCrier.Infrastructure.Authorities;
using TownCrier.Infrastructure.Cosmos;
using TownCrier.Infrastructure.DecisionAlerts;
using TownCrier.Infrastructure.DeviceRegistrations;
using TownCrier.Infrastructure.Notifications;
using TownCrier.Infrastructure.Observability;
using TownCrier.Infrastructure.PlanIt;
using TownCrier.Infrastructure.PlanningApplications;
using TownCrier.Infrastructure.Polling;
using TownCrier.Infrastructure.SavedApplications;
using TownCrier.Infrastructure.ServiceBus;
using TownCrier.Infrastructure.UserProfiles;
using TownCrier.Infrastructure.WatchZones;
using TownCrier.Worker;

var builder = Host.CreateApplicationBuilder(args);

var aiConnectionString = builder.Configuration["APPLICATIONINSIGHTS_CONNECTION_STRING"];
var hasAppInsights = !string.IsNullOrEmpty(aiConnectionString);

// Strip the default ILoggerProvider set (Console, Debug, EventSource, EventLog).
// OpenTelemetry below is the sole logging provider — the Azure Monitor exporter
// ships structured logs to App Insights AppTraces, where incident debugging
// happens. The console provider is removed because every ILogger call would
// otherwise duplicate into stdout and into Container Apps' priced
// ContainerAppConsoleLogs_CL Log Analytics table (~0.3 GB/day duplicate
// ingestion). See bead tc-lve1.
builder.Logging.ClearProviders();

var otel = builder.Services.AddOpenTelemetry()
    .ConfigureResource(resource => resource.AddService("town-crier-worker"))
    .WithTracing(tracing =>
    {
        tracing
            .AddProcessor<SuccessfulCosmosDependencyFilter>()
            .AddHttpClientInstrumentation(options => options.RecordException = true)
            .AddSource(PollingInstrumentation.ActivitySourceName)
            .AddSource(CosmosInstrumentation.ActivitySourceName);
    })
    .WithMetrics(metrics =>
    {
        metrics
            .AddHttpClientInstrumentation()
            .AddMeter(PollingMetrics.MeterName)
            .AddMeter(ApiMetrics.MeterName)
            .AddMeter(CosmosInstrumentation.MeterName)
            .AddMeter(PlanItInstrumentation.MeterName);
    })
    .WithLogging(
        configureBuilder: null,
        configureOptions: logging =>
        {
            logging.IncludeFormattedMessage = true;
            logging.IncludeScopes = true;
        });

if (hasAppInsights)
{
    otel.UseAzureMonitorExporter(o =>
    {
        o.ConnectionString = aiConnectionString;

        // Azure Monitor Exporter 1.6.0+ defaults to RateLimitedSampler at 5 TPS,
        // which drops most dependency spans under burst traffic (e.g., Cosmos polling
        // with 900+ calls). Set TracesPerSecond=null to use ApplicationInsightsSampler
        // with SamplingRatio=1.0 for 100% fixed-percentage sampling, ensuring all
        // outbound calls appear in the App Insights dependencies table.
        o.SamplingRatio = 1.0f;
        o.TracesPerSecond = null;
    });
}

builder.Services.Configure<MetricReaderOptions>(o =>
{
    o.PeriodicExportingMetricReaderOptions = new PeriodicExportingMetricReaderOptions
    {
        ExportIntervalMilliseconds = 5_000,
    };
});

builder.Services.AddCosmosRestClient(builder.Configuration);

builder.Services.AddSingleton<IPlanningApplicationRepository, CosmosPlanningApplicationRepository>();
builder.Services.AddSingleton<IWatchZoneRepository, CosmosWatchZoneRepository>();
builder.Services.AddSingleton<IPollStateStore, CosmosPollStateStore>();
builder.Services.AddSingleton<IWatchZoneActiveAuthorityProvider, WatchZoneActiveAuthorityProvider>();
builder.Services.AddSingleton<IAllAuthorityIdProvider, AllAuthorityIdProvider>();
builder.Services.AddSingleton<IAuthorityProvider, StaticAuthorityProvider>();
builder.Services.AddSingleton<ICycleSelector, MinuteBasedCycleSelector>();
builder.Services.AddSingleton<IActiveAuthorityProvider, CycleAlternatingAuthorityProvider>();
builder.Services.AddSingleton<INotificationRepository, CosmosNotificationRepository>();
builder.Services.AddSingleton<IUserProfileRepository, CosmosUserProfileRepository>();
builder.Services.AddSingleton<IDeviceRegistrationRepository, CosmosDeviceRegistrationRepository>();
builder.Services.AddSingleton<IDecisionAlertRepository, CosmosDecisionAlertRepository>();
builder.Services.AddSingleton<ISavedApplicationRepository, CosmosSavedApplicationRepository>();
builder.Services.AddSingleton<IPushNotificationSender, NoOpPushNotificationSender>();

// Dormant account cleanup needs the full delete-cascade pipeline. Auth0
// management is configured identically to the web host — prefer the real
// client when M2M credentials are present, fall back to a no-op otherwise.
var workerAuth0M2mClientId = builder.Configuration["Auth0:M2M:ClientId"];
var workerAuth0M2mClientSecret = builder.Configuration["Auth0:M2M:ClientSecret"];
var workerAuth0Domain = builder.Configuration["Auth0:Domain"];
if (!string.IsNullOrEmpty(workerAuth0M2mClientId)
    && !string.IsNullOrEmpty(workerAuth0M2mClientSecret)
    && !string.IsNullOrEmpty(workerAuth0Domain))
{
    builder.Services.AddHttpClient<IAuth0ManagementClient, Auth0ManagementClient>((httpClient, sp) =>
        new Auth0ManagementClient(httpClient, workerAuth0Domain, workerAuth0M2mClientId, workerAuth0M2mClientSecret));
}
else
{
    builder.Services.AddSingleton<IAuth0ManagementClient, NoOpAuth0ManagementClient>();
}

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

var planItThrottle = new PlanItThrottleOptions();
builder.Configuration.GetSection("PlanIt:Throttle").Bind(planItThrottle);
builder.Services.AddSingleton(planItThrottle);

var planItRetry = new PlanItRetryOptions();
builder.Configuration.GetSection("PlanIt:Retry").Bind(planItRetry);
builder.Services.AddSingleton(planItRetry);

#pragma warning disable S1075 // Hardcoded URI is a sensible default
var planItBaseUrl = builder.Configuration["PlanIt:BaseUrl"] ?? "https://www.planit.org.uk/";
#pragma warning restore S1075
builder.Services.AddHttpClient<IPlanItClient, PlanItClient>(client =>
{
    client.BaseAddress = new Uri(planItBaseUrl);
});

// Default 240 s leaves 60 s headroom inside the 300 s Service Bus message
// lock for the orchestrator's publish-next + complete. Set to 0 to disable
// (diagnostics only). See docs/specs/poll-handler-soft-budget.md.
var handlerBudgetSeconds = builder.Configuration.GetValue<int?>("POLLING_HANDLER_BUDGET_SECONDS") ?? 240;
var pollingOptions = new PollingOptions
{
    // Default 3 pages = 300 apps max per authority per cycle. Bounds the
    // per-authority rate budget so a backlogged authority can't monopolise a
    // seed cycle. Null disables the cap (unbounded pagination). See bd tc-l77h.
    MaxPagesPerAuthorityPerCycle = builder.Configuration.GetValue<int?>("Polling:MaxPagesPerAuthorityPerCycle") ?? 3,
    HandlerBudget = handlerBudgetSeconds > 0 ? TimeSpan.FromSeconds(handlerBudgetSeconds) : null,
};
builder.Services.AddSingleton(pollingOptions);

builder.Services.AddTransient<PollPlanItCommandHandler>();

// Service Bus-driven adaptive polling chain (WORKER_MODE=poll-sb). Registers the
// REST client, the ServiceBusPollTriggerQueue adapter, the scheduler, and the
// orchestrator. Safe to register even when the safety-net timer mode is in use —
// the services are only resolved by the poll-sb branch below.
builder.Services.AddServiceBusRestClient(builder.Configuration);
builder.Services.AddPollingInfrastructure(builder.Configuration);

builder.Services.AddSingleton<GenerateWeeklyDigestsCommandHandler>();
builder.Services.AddSingleton<GenerateHourlyDigestsCommandHandler>();
builder.Services.AddTransient<DeleteUserProfileCommandHandler>();
builder.Services.AddTransient<DormantAccountCleanupCommandHandler>();

using var host = builder.Build();

// Start hosted services so the ExporterRegistrationHostedService runs.
// This service wires the Azure Monitor trace and log exporters into the
// TracerProvider and LoggerProvider — without it, spans and log-based
// exceptions are silently dropped.
await host.StartAsync().ConfigureAwait(false);

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
        {
            using var activity = PollingInstrumentation.Source.StartActivity("Polling Cycle");
            try
            {
                var cycleStart = Stopwatch.GetTimestamp();

                var cycleSelector = host.Services.GetRequiredService<ICycleSelector>();
                var cycleType = cycleSelector.GetCurrent();
                var cycleTypeValue = cycleType.ToTelemetryValue();
                activity?.SetTag("cycle.type", cycleTypeValue);

                WorkerLog.PollCycleStarting(logger);

                // Bound the poll cycle to (replicaTimeout - grace) so the handler can unwind
                // cleanly before Container Apps SIGTERMs us at replicaTimeout. Without this,
                // seed cycles that legitimately take the full window get marked Failed even
                // though they ingested thousands of applications. See bd tc-qdtu.
                //
                // Defaults mirror infra/EnvironmentStack.cs (replicaTimeout=600s). The env
                // var overrides exist so infra changes don't silently drift from worker
                // behaviour — redeploy with both in sync.
                var replicaTimeoutSeconds = builder.Configuration.GetValue<int?>("POLL_REPLICA_TIMEOUT_SECONDS") ?? 600;
                var shutdownGraceSeconds = builder.Configuration.GetValue<int?>("POLL_SHUTDOWN_GRACE_SECONDS") ?? 30;
                var cycleBudget = TimeSpan.FromSeconds(Math.Max(1, replicaTimeoutSeconds - shutdownGraceSeconds));

                using var cycleCts = new CancellationTokenSource(cycleBudget);

                var pollHandler = host.Services.GetRequiredService<PollPlanItCommandHandler>();
                var result = await pollHandler.HandleAsync(new PollPlanItCommand(), cycleCts.Token)
                    .ConfigureAwait(false);

                PollingMetrics.CycleDuration.Record(Stopwatch.GetElapsedTime(cycleStart).TotalMilliseconds);

                activity?.SetTag("polling.authorities_polled", result.AuthoritiesPolled);
                activity?.SetTag("polling.applications_ingested", result.ApplicationCount);
                activity?.SetTag("polling.termination", result.TerminationReason.ToTelemetryValue());
                activity?.SetTag("polling.authority_errors", result.AuthorityErrors);

                WorkerLog.PollCycleCompleted(logger, result.ApplicationCount, result.AuthoritiesPolled);

                // Safety-net reseed (bd tc-tdgf): after the cron poll, re-seed the
                // Service Bus trigger queue if it's empty so the adaptive SB-coordinated
                // cycle self-heals. Best-effort — the bootstrapper absorbs any probe or
                // publish failure so the safety-net's primary job (polling) is never
                // blocked. Skipped when the lease was held by another replica — that
                // holder is responsible for the SB chain.
                if (result.TerminationReason != PollTerminationReason.LeaseHeld)
                {
                    var bootstrapper = host.Services.GetRequiredService<PollTriggerBootstrapper>();
                    var bootstrapResult = await bootstrapper.TryBootstrapAsync(cycleCts.Token)
                        .ConfigureAwait(false);
                    activity?.SetTag("polling.safety_net.bootstrap_published", bootstrapResult.Published);
                    activity?.SetTag("polling.safety_net.bootstrap_probe_failed", bootstrapResult.ProbeFailed);
                }

                // Exit-code semantics redefined for bd tc-qdtu:
                //   exit 0 when ApplicationCount > 0  (we did useful work), OR
                //   exit 0 when AuthorityErrors == 0  (clean pass, even if nothing new).
                //   exit 1 only when we did NO useful work AND hit per-authority errors.
                // Rate-limited stops remain exit 0 — PlanIt throttling is an expected
                // outcome for seed cycles, not a worker failure.
                if (result.ApplicationCount == 0 && result.AuthorityErrors > 0)
                {
                    exitCode = 1;
                }
            }
#pragma warning disable CA1031 // Worker must return exit code on any failure
            catch (Exception ex)
#pragma warning restore CA1031
            {
                activity?.AddException(ex);
                activity?.SetStatus(ActivityStatusCode.Error, ex.Message);
                PollingMetrics.PollFailures.Add(1);
                WorkerLog.PollCycleFailed(logger, ex);
                exitCode = 1;
            }

            break;
        }

    case "poll-sb":
        {
            // Service-Bus-triggered adaptive polling loop. Receives one trigger
            // message, runs the handler, publishes the next trigger (with a
            // scheduled enqueue time), then acks. Publish-before-ack ordering is
            // load-bearing for crash safety — see PollTriggerOrchestrator.
            using var sbActivity = PollingInstrumentation.Source.StartActivity("Polling Cycle (SB)");
            try
            {
                WorkerLog.PollCycleStarting(logger);

                var replicaTimeoutSeconds = builder.Configuration.GetValue<int?>("POLL_REPLICA_TIMEOUT_SECONDS") ?? 600;
                var shutdownGraceSeconds = builder.Configuration.GetValue<int?>("POLL_SHUTDOWN_GRACE_SECONDS") ?? 30;
                var cycleBudget = TimeSpan.FromSeconds(Math.Max(1, replicaTimeoutSeconds - shutdownGraceSeconds));

                using var cycleCts = new CancellationTokenSource(cycleBudget);

                var orchestrator = host.Services.GetRequiredService<PollTriggerOrchestrator>();
                var runResult = await orchestrator.RunOnceAsync(cycleCts.Token).ConfigureAwait(false);

                sbActivity?.SetTag("polling.sb.message_received", runResult.MessageReceived);
                sbActivity?.SetTag("polling.sb.published_next", runResult.PublishedNext);

                if (runResult.PollResult is { } pollResult)
                {
                    sbActivity?.SetTag("polling.authorities_polled", pollResult.AuthoritiesPolled);
                    sbActivity?.SetTag("polling.applications_ingested", pollResult.ApplicationCount);
                    sbActivity?.SetTag("polling.termination", pollResult.TerminationReason.ToTelemetryValue());
                    sbActivity?.SetTag("polling.authority_errors", pollResult.AuthorityErrors);

                    WorkerLog.PollCycleCompleted(logger, pollResult.ApplicationCount, pollResult.AuthoritiesPolled);

                    // Same exit-code semantics as the safety-net timer branch: only
                    // exit 1 when the run did no useful work AND hit authority errors.
                    if (pollResult.ApplicationCount == 0 && pollResult.AuthorityErrors > 0)
                    {
                        exitCode = 1;
                    }
                }
                else
                {
                    WorkerLog.PollCycleCompleted(logger, 0, 0);
                }
            }
#pragma warning disable CA1031 // Worker must return exit code on any failure
            catch (Exception ex)
#pragma warning restore CA1031
            {
                sbActivity?.AddException(ex);
                sbActivity?.SetStatus(ActivityStatusCode.Error, ex.Message);
                PollingMetrics.PollFailures.Add(1);
                WorkerLog.PollCycleFailed(logger, ex);
                exitCode = 1;
            }

            break;
        }

    case "digest":
        {
            using var digestActivity = PollingInstrumentation.Source.StartActivity("Digest Cycle");
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
                digestActivity?.AddException(ex);
                digestActivity?.SetStatus(ActivityStatusCode.Error, ex.Message);
                WorkerLog.DigestCycleFailed(logger, ex);
                exitCode = 1;
            }

            break;
        }

    case "hourly-digest":
        {
            using var hourlyDigestActivity = PollingInstrumentation.Source.StartActivity("Hourly Digest Cycle");
            try
            {
                WorkerLog.HourlyDigestCycleStarting(logger);

                var hourlyDigestHandler = host.Services.GetRequiredService<GenerateHourlyDigestsCommandHandler>();
                await hourlyDigestHandler.HandleAsync(new GenerateHourlyDigestsCommand(), CancellationToken.None)
                    .ConfigureAwait(false);

                WorkerLog.HourlyDigestCycleCompleted(logger);
            }
#pragma warning disable CA1031 // Worker must return exit code on any failure
            catch (Exception ex)
#pragma warning restore CA1031
            {
                hourlyDigestActivity?.AddException(ex);
                hourlyDigestActivity?.SetStatus(ActivityStatusCode.Error, ex.Message);
                WorkerLog.HourlyDigestCycleFailed(logger, ex);
                exitCode = 1;
            }

            break;
        }

    case "dormant-cleanup":
        {
            // Daily job that enforces UK GDPR Art. 5(1)(e) storage limitation —
            // deletes accounts inactive for 12+ months (per privacy policy).
            using var dormantActivity = PollingInstrumentation.Source.StartActivity("Dormant Cleanup Cycle");
            try
            {
                WorkerLog.DormantCleanupStarting(logger);

                var cleanupHandler = host.Services.GetRequiredService<DormantAccountCleanupCommandHandler>();
                var timeProvider = host.Services.GetRequiredService<TimeProvider>();
                var cleanupResult = await cleanupHandler
                    .HandleAsync(new DormantAccountCleanupCommand(timeProvider.GetUtcNow()), CancellationToken.None)
                    .ConfigureAwait(false);

                dormantActivity?.SetTag("dormant_cleanup.deleted_count", cleanupResult.DeletedCount);
                WorkerLog.DormantCleanupCompleted(logger, cleanupResult.DeletedCount);
            }
#pragma warning disable CA1031 // Worker must return exit code on any failure
            catch (Exception ex)
#pragma warning restore CA1031
            {
                dormantActivity?.AddException(ex);
                dormantActivity?.SetStatus(ActivityStatusCode.Error, ex.Message);
                WorkerLog.DormantCleanupFailed(logger, ex);
                exitCode = 1;
            }

            break;
        }

    default:
        WorkerLog.UnknownWorkerMode(logger, mode);
        exitCode = 1;
        break;
}

// Stop hosted services (mirrors the StartAsync above).
await host.StopAsync().ConfigureAwait(false);

// Force-flush OpenTelemetry before the short-lived process exits.
// The Azure Monitor exporter batches on 5 s intervals; without this,
// the worker may terminate before the final batch is sent.
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
