using System.Diagnostics;
using Azure.Monitor.OpenTelemetry.Exporter;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.TestHost;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Options;
using OpenTelemetry;
using OpenTelemetry.Trace;

namespace TownCrier.Web.Tests.Observability;

public sealed class DependencyTracingTests
{
#pragma warning disable S1075 // Test URIs are intentionally hardcoded
    private const string FakeDependencyUrl = "http://localhost:19999/fake-dependency";
#pragma warning restore S1075

    [Test]
    [Retry(3)]
    public async Task Should_ExportClientSpan_When_OutboundHttpCallIsMade()
    {
        // Arrange -- configure a fake App Insights connection string so the
        // Azure Monitor exporter (and its sampler) are registered, matching
        // the production code path in Program.cs. The sampler must not drop
        // Client spans for them to appear in the App Insights dependencies table.
        var exportedActivities = new List<Activity>();
        await using var baseFactory = new TestWebApplicationFactory();
        await using var factory = baseFactory.WithWebHostBuilder(builder =>
        {
            builder.UseSetting(
                "APPLICATIONINSIGHTS_CONNECTION_STRING",
                "InstrumentationKey=00000000-0000-0000-0000-000000000000;IngestionEndpoint=https://localhost/");

            builder.ConfigureTestServices(services =>
            {
                services.AddOpenTelemetry()
                    .WithTracing(tracing =>
                    {
                        tracing.AddInMemoryExporter(exportedActivities);
                    });
            });
        });
        using var client = factory.CreateClient();

        // Act -- create an HttpClient from the DI container to simulate an
        // outbound call within the traced pipeline.
        var innerHttpClientFactory = factory.Services.GetRequiredService<IHttpClientFactory>();
        using var innerClient = innerHttpClientFactory.CreateClient();

        try
        {
            // This call will fail (no real server) but should still create a Client span
            await innerClient.GetAsync(new Uri(FakeDependencyUrl));
        }
        catch (HttpRequestException)
        {
            // Expected -- the target doesn't exist, but the span should still be created
        }

        // Force flush to ensure all spans are exported
        var tracerProvider = factory.Services.GetRequiredService<TracerProvider>();
        tracerProvider.ForceFlush();

        // Assert -- verify a Client span was exported (maps to App Insights dependencies table).
        // If the Azure Monitor sampler is dropping these spans, this test fails.
        var clientSpan = exportedActivities.Find(a => a.Kind == ActivityKind.Client);
        await Assert.That(clientSpan).IsNotNull()
            .Because("HTTP Client spans must be exported for the App Insights dependencies table");
    }

    [Test]
    [Retry(3)]
    public async Task Should_IncludeHttpAttributesOnClientSpan_When_OutboundHttpCallIsMade()
    {
        // Arrange
        var exportedActivities = new List<Activity>();
        await using var baseFactory = new TestWebApplicationFactory();
        await using var factory = baseFactory.WithWebHostBuilder(builder =>
        {
            builder.UseSetting(
                "APPLICATIONINSIGHTS_CONNECTION_STRING",
                "InstrumentationKey=00000000-0000-0000-0000-000000000000;IngestionEndpoint=https://localhost/");

            builder.ConfigureTestServices(services =>
            {
                services.AddOpenTelemetry()
                    .WithTracing(tracing =>
                    {
                        tracing.AddInMemoryExporter(exportedActivities);
                    });
            });
        });

        var innerHttpClientFactory = factory.Services.GetRequiredService<IHttpClientFactory>();
        using var innerClient = innerHttpClientFactory.CreateClient();

        try
        {
            await innerClient.GetAsync(new Uri(FakeDependencyUrl));
        }
        catch (HttpRequestException)
        {
            // Expected
        }

        var tracerProvider = factory.Services.GetRequiredService<TracerProvider>();
        tracerProvider.ForceFlush();

        // Assert -- the Client span must have HTTP semantic attributes that the
        // Azure Monitor exporter uses to populate the dependencies table.
        var clientSpan = exportedActivities.Find(a => a.Kind == ActivityKind.Client);
        await Assert.That(clientSpan).IsNotNull();

        // Check for HTTP method attribute (new semantic conventions: http.request.method)
        var httpMethod = clientSpan!.GetTagItem("http.request.method")
            ?? clientSpan.GetTagItem("http.method");
        await Assert.That(httpMethod).IsNotNull()
            .Because("Client span must include http.request.method for App Insights dependency mapping");

        // Check for server address (used for dependency target in App Insights)
        var serverAddress = clientSpan.GetTagItem("server.address")
            ?? clientSpan.GetTagItem("http.host")
            ?? clientSpan.GetTagItem("net.peer.name");
        await Assert.That(serverAddress).IsNotNull()
            .Because("Client span must include server.address for App Insights dependency target");
    }

    [Test]
    public async Task Should_UseFixedPercentageSampling_When_AzureMonitorExporterIsConfigured()
    {
        // Arrange -- Azure Monitor Exporter 1.6.0+ defaults to RateLimitedSampler
        // at 5 TPS (TracesPerSecond=5.0), which drops most spans under burst traffic
        // (e.g., Cosmos polling cycles with 900+ calls). Our configuration must set
        // TracesPerSecond=null to use ApplicationInsightsSampler with SamplingRatio=1.0
        // for 100% fixed-percentage sampling instead.
        await using var baseFactory = new TestWebApplicationFactory();
        await using var factory = baseFactory.WithWebHostBuilder(builder =>
        {
            builder.UseSetting(
                "APPLICATIONINSIGHTS_CONNECTION_STRING",
                "InstrumentationKey=00000000-0000-0000-0000-000000000000;IngestionEndpoint=https://localhost/");
        });

        // Act -- resolve the configured options from DI
        var optionsMonitor = factory.Services.GetRequiredService<IOptionsMonitor<AzureMonitorExporterOptions>>();
        var options = optionsMonitor.CurrentValue;

        // Assert -- TracesPerSecond must be null so the exporter uses
        // ApplicationInsightsSampler(SamplingRatio) instead of RateLimitedSampler(TPS).
        // SamplingRatio must be 1.0 for 100% trace capture.
        await Assert.That(options.TracesPerSecond).IsNull()
            .Because(
                "TracesPerSecond must be null to disable the RateLimitedSampler; " +
                "the default 5.0 TPS drops most dependency spans under burst traffic");

        await Assert.That(options.SamplingRatio).IsEqualTo(1.0f)
            .Because(
                "SamplingRatio must be 1.0 (100%) so all dependency spans reach " +
                "the App Insights dependencies table for full correlation");
    }
}
