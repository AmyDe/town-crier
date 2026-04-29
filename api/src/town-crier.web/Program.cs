using Azure.Monitor.OpenTelemetry.Exporter;
using OpenTelemetry;
using OpenTelemetry.Logs;
using OpenTelemetry.Metrics;
using OpenTelemetry.Resources;
using OpenTelemetry.Trace;
using TownCrier.Application.Observability;
using TownCrier.Infrastructure.Observability;
using TownCrier.Web;
using TownCrier.Web.Extensions;

var builder = WebApplication.CreateSlimBuilder(args);

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
    .ConfigureResource(resource => resource.AddService("town-crier-api"))
    .WithTracing(tracing =>
    {
        tracing
            .AddProcessor<SuccessfulCosmosDependencyFilter>()
            .AddAspNetCoreInstrumentation(options => options.RecordException = true)
            .AddHttpClientInstrumentation(options => options.RecordException = true)
            .AddSource(PollingInstrumentation.ActivitySourceName)
            .AddSource(CosmosInstrumentation.ActivitySourceName);
    })
    .WithMetrics(metrics =>
    {
        metrics
            .AddAspNetCoreInstrumentation()
            .AddHttpClientInstrumentation()
            .AddMeter(ApiMetrics.MeterName)
            .AddMeter(PollingMetrics.MeterName)
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

var allowedOrigins = builder.Configuration.GetSection("Cors:AllowedOrigins").Get<string[]>()
    ?? ["http://localhost:5173"];
builder.Services.AddCors(options =>
{
    options.AddDefaultPolicy(policy =>
    {
        policy.WithOrigins(allowedOrigins)
            .AllowAnyHeader()
            .AllowAnyMethod();
    });
});

builder.Services.ConfigureHttpJsonOptions(options =>
{
    options.SerializerOptions.TypeInfoResolverChain.Insert(0, AppJsonSerializerContext.Default);
});

builder.Services.AddInfrastructureServices(builder.Configuration);
builder.Services.AddApplicationServices(builder.Configuration);
builder.Services.AddAuthenticationServices(builder.Configuration);

var app = builder.Build();

app.UseMiddlewarePipeline();
app.MapAllEndpoints();

await app.RunAsync().ConfigureAwait(false);
