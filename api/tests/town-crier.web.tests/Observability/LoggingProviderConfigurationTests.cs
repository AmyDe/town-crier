using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Logging.Console;
using OpenTelemetry.Logs;

namespace TownCrier.Web.Tests.Observability;

public sealed class LoggingProviderConfigurationTests
{
    [Test]
    public async Task Should_NotRegisterConsoleLoggerProvider_When_HostIsBuilt()
    {
        // Arrange — build the web host exactly as Program.cs configures it.
        // The default ASP.NET Core host registers ConsoleLoggerProvider, which
        // duplicates every ILogger call into stdout (and therefore into the
        // ContainerAppConsoleLogs_CL Log Analytics table). OpenTelemetry already
        // ships the same data — richer and structured — to App Insights AppTraces,
        // so the console provider is pure duplicate ingestion cost.
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        // Act — enumerate all registered ILoggerProvider implementations.
        var providers = factory.Services.GetServices<ILoggerProvider>().ToList();

        // Assert — ConsoleLoggerProvider must not be present. Builder.Logging.ClearProviders()
        // followed by re-registering only OpenTelemetry is the supported pattern.
        await Assert.That(providers.Exists(p => p is ConsoleLoggerProvider)).IsFalse()
            .Because(
                "ConsoleLoggerProvider duplicates OpenTelemetry log output to stdout, " +
                "doubling Log Analytics ingestion cost (see bead tc-lve1).");
    }

    [Test]
    public async Task Should_RegisterOpenTelemetryLoggerProvider_When_HostIsBuilt()
    {
        // Arrange — clearing providers must not also remove the OpenTelemetry
        // logging pipeline; AppTraces remains the sole structured-log surface.
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        // Act
        var providers = factory.Services.GetServices<ILoggerProvider>().ToList();

        // Assert — the OpenTelemetry provider must still be present so
        // App Insights AppTraces continues to receive structured logs.
        await Assert.That(providers.Exists(p => p is OpenTelemetryLoggerProvider)).IsTrue()
            .Because(
                "OpenTelemetry logging must remain registered after ClearProviders so " +
                "App Insights AppTraces continues to receive structured logs.");
    }
}
