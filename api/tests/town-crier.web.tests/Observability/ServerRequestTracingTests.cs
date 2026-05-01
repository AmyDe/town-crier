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
    // Maximum time to wait for the ASP.NET Core server activity to be ended
    // and exported to the in-memory exporter. The activity is stopped by the
    // framework as the request pipeline unwinds, which can race with HttpClient
    // returning from GetAsync — the response body completes before the
    // server-side activity.Stop() is observed by the exporter.
    private static readonly TimeSpan SpanWaitTimeout = TimeSpan.FromSeconds(5);

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

        // Assert -- verify a Server span was exported (maps to App Insights requests table)
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);

        var tracerProvider = factory.Services.GetRequiredService<TracerProvider>();
        var serverSpan = await WaitForSpanAsync(
            exportedActivities, tracerProvider, a => a.Kind == ActivityKind.Server);
        await Assert.That(serverSpan).IsNotNull()
            .Because("ASP.NET Core server spans must be exported for the App Insights requests table");
    }

    [Test]
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

        // Assert -- the response should be 500 and the server span must carry the exception
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.InternalServerError);

        // Wait for the server span to be ended and exported. The activity is stopped
        // by the ASP.NET Core hosting layer as the request unwinds, which can race
        // with HttpClient returning. Poll with bounded timeout for a span that has
        // both Server kind AND the exception event recorded by ErrorResponseMiddleware,
        // so we don't observe a half-built activity that has been exported but whose
        // exception hasn't been attached yet.
        var tracerProvider = factory.Services.GetRequiredService<TracerProvider>();
        var serverSpan = await WaitForSpanAsync(
            exportedActivities,
            tracerProvider,
            a => a.Kind == ActivityKind.Server
                && a.Events.Any(e => e.Name == "exception"));
        await Assert.That(serverSpan).IsNotNull()
            .Because("a server span with an exception event must be exported when the request fails");

        var exceptionEvent = serverSpan!.Events.First(e => e.Name == "exception");
        var exceptionType = exceptionEvent.Tags
            .FirstOrDefault(t => t.Key == "exception.type").Value as string;
        await Assert.That(exceptionType).IsEqualTo("System.InvalidOperationException")
            .Because("ErrorResponseMiddleware must record the exception type on the span for App Insights");

        await Assert.That(serverSpan.Status).IsEqualTo(ActivityStatusCode.Error);
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

        // Assert -- the server span must have HTTP semantic attributes that the
        // Azure Monitor exporter uses to populate the requests table. Without these
        // attributes, spans are exported but result in empty/malformed RequestData.
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);

        var tracerProvider = factory.Services.GetRequiredService<TracerProvider>();
        var serverSpan = await WaitForSpanAsync(
            exportedActivities, tracerProvider, a => a.Kind == ActivityKind.Server);
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

    // Polls the in-memory exporter until a matching activity has been recorded, or
    // SpanWaitTimeout elapses. ASP.NET Core ends the server activity asynchronously
    // as the request pipeline unwinds — that can race with HttpClient returning from
    // GetAsync, leaving a window where exportedActivities is briefly empty. We call
    // ForceFlush on every iteration so SimpleActivityExportProcessor drains any
    // pending activities synchronously, and snapshot the list under a lock to avoid
    // racing with concurrent writes from the exporter thread.
    private static async Task<Activity?> WaitForSpanAsync(
        List<Activity> exportedActivities,
        TracerProvider tracerProvider,
        Func<Activity, bool> predicate)
    {
        var deadline = DateTimeOffset.UtcNow + SpanWaitTimeout;
        while (true)
        {
            tracerProvider.ForceFlush(timeoutMilliseconds: 100);

            Activity[] snapshot;
            lock (exportedActivities)
            {
                snapshot = [.. exportedActivities];
            }

            var match = Array.Find(snapshot, new Predicate<Activity>(predicate));
            if (match is not null)
            {
                return match;
            }

            if (DateTimeOffset.UtcNow >= deadline)
            {
                return null;
            }

            await Task.Delay(25);
        }
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

        public Task<UserProfilePage> ListAsync(
            string? emailSearch, int pageSize, string? continuationToken, CancellationToken ct) =>
            throw new InvalidOperationException("Simulated repository failure");

        public Task<IReadOnlyList<UserProfile>> GetDormantAsync(DateTimeOffset cutoff, CancellationToken ct) =>
            throw new InvalidOperationException("Simulated repository failure");
    }
}
