using System.Diagnostics;
using System.Net;
using System.Net.Http.Headers;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.TestHost;
using Microsoft.Extensions.DependencyInjection;
using OpenTelemetry;
using OpenTelemetry.Trace;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;
using TownCrier.Web.Tests.Auth;

namespace TownCrier.Web.Tests.Observability;

public sealed class ServerRequestTracingTests
{
    [Test]
    [Retry(3)]
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
    [Retry(3)]
    public async Task Should_ExportExceptionEvent_When_EndpointThrows()
    {
        // Arrange -- replace the user profile repository with one that always throws,
        // so GET /v1/me triggers an unhandled exception through ErrorResponseMiddleware.
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

                services.AddSingleton<IUserProfileRepository>(
                    new ThrowingUserProfileRepository());
            });
        });
        using var client = factory.CreateClient();
        client.DefaultRequestHeaders.Authorization =
            new AuthenticationHeaderValue("Bearer", TestJwtToken.Generate());

        // Act
        using var response = await client.GetAsync(new Uri("/v1/me", UriKind.Relative));

        var tracerProvider = factory.Services.GetRequiredService<TracerProvider>();
        tracerProvider.ForceFlush();

        // Assert -- the response should be 500 and the server span must carry the exception
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.InternalServerError);

        var serverSpan = exportedActivities.Find(a => a.Kind == ActivityKind.Server);
        await Assert.That(serverSpan).IsNotNull()
            .Because("a server span must be exported even when the request fails");

        var exceptionEvent = serverSpan!.Events.FirstOrDefault(e => e.Name == "exception");
        await Assert.That(exceptionEvent.Name).IsEqualTo("exception")
            .Because("ErrorResponseMiddleware must record the exception on the span for App Insights");

        var exceptionType = exceptionEvent.Tags
            .FirstOrDefault(t => t.Key == "exception.type").Value as string;
        await Assert.That(exceptionType).IsEqualTo("System.InvalidOperationException");

        await Assert.That(serverSpan.Status).IsEqualTo(ActivityStatusCode.Error);
    }

    [Test]
    [Retry(3)]
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

    private sealed class ThrowingUserProfileRepository : IUserProfileRepository
    {
        public Task<UserProfile?> GetByUserIdAsync(string userId, CancellationToken ct) =>
            throw new InvalidOperationException("Simulated repository failure");

        public Task<UserProfile?> GetByEmailAsync(string email, CancellationToken ct) =>
            throw new InvalidOperationException("Simulated repository failure");

        public Task<IReadOnlyList<UserProfile>> GetAllByTierAsync(SubscriptionTier tier, CancellationToken ct) =>
            throw new InvalidOperationException("Simulated repository failure");

        public Task<IReadOnlyList<UserProfile>> GetAllByDigestDayAsync(DayOfWeek digestDay, CancellationToken ct) =>
            throw new InvalidOperationException("Simulated repository failure");

        public Task<UserProfile?> GetByOriginalTransactionIdAsync(string originalTransactionId, CancellationToken ct) =>
            throw new InvalidOperationException("Simulated repository failure");

        public Task SaveAsync(UserProfile profile, CancellationToken ct) =>
            throw new InvalidOperationException("Simulated repository failure");

        public Task DeleteAsync(string userId, CancellationToken ct) =>
            throw new InvalidOperationException("Simulated repository failure");
    }
}
