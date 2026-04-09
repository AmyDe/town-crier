using System.Diagnostics;
using System.Net;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.TestHost;
using Microsoft.Extensions.DependencyInjection;
using OpenTelemetry;
using OpenTelemetry.Trace;

namespace TownCrier.Web.Tests.Observability;

public sealed class ServerRequestTracingTests
{
    [Test]
    public async Task Should_ExportServerSpan_When_HttpRequestIsHandled()
    {
        // Arrange -- configure a fake App Insights connection string so the
        // Azure Monitor trace exporter (and its sampler) are registered, matching
        // the production code path in Program.cs.
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

        // Act
        using var response = await client.GetAsync(new Uri("/health", UriKind.Relative));

        // Force flush to ensure all spans are exported
        var tracerProvider = factory.Services.GetRequiredService<TracerProvider>();
        tracerProvider.ForceFlush();

        // Assert -- verify a Server span was exported (maps to App Insights requests table)
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);

        var serverSpan = exportedActivities.Find(a => a.Kind == ActivityKind.Server);
        await Assert.That(serverSpan).IsNotNull()
            .Because("ASP.NET Core server spans must be exported for the App Insights requests table");
    }

    [Test]
    public async Task Should_IncludeHttpAttributesOnServerSpan_When_HttpRequestIsHandled()
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
        using var client = factory.CreateClient();

        // Act
        using var response = await client.GetAsync(new Uri("/v1/health", UriKind.Relative));

        var tracerProvider = factory.Services.GetRequiredService<TracerProvider>();
        tracerProvider.ForceFlush();

        // Assert -- the server span must have HTTP semantic attributes that the
        // Azure Monitor exporter uses to populate the requests table. Without these
        // attributes, spans are exported but result in empty/malformed RequestData.
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);

        var serverSpan = exportedActivities.Find(a => a.Kind == ActivityKind.Server);
        await Assert.That(serverSpan).IsNotNull();

        // Check for HTTP method attribute (new semantic conventions: http.request.method)
        var httpMethod = serverSpan!.GetTagItem("http.request.method")
            ?? serverSpan.GetTagItem("http.method");
        await Assert.That(httpMethod).IsNotNull()
            .Because("server span must include http.request.method for App Insights request mapping");

        // Check for HTTP status code attribute
        var statusCode = serverSpan.GetTagItem("http.response.status_code")
            ?? serverSpan.GetTagItem("http.status_code");
        await Assert.That(statusCode).IsNotNull()
            .Because("server span must include http.response.status_code for App Insights request mapping");

        // Check for URL route attribute (used for request name in App Insights)
        var route = serverSpan.GetTagItem("http.route")
            ?? serverSpan.GetTagItem("url.path");
        await Assert.That(route).IsNotNull()
            .Because("server span must include http.route for App Insights request naming");
    }
}
