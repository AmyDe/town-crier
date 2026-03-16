using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging;

namespace TownCrier.Web.Tests.Observability;

public sealed class RequestLoggingMiddlewareTests
{
    [Test]
    public async Task Should_LogRequestWithStatusAndDuration_When_RequestCompletes()
    {
        // Arrange
        using var logSink = new SpyLoggerProvider();
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.WithWebHostBuilder(builder =>
        {
            builder.ConfigureServices(services =>
            {
                services.AddLogging(logging =>
                {
                    logging.AddProvider(logSink);
                });
            });
        }).CreateClient();

        // Act
        using var response = await client.GetAsync(new Uri("/health", UriKind.Relative));

        // Assert
        var logEntry = logSink.Entries.FirstOrDefault(e =>
            e.Message.Contains("/health", StringComparison.Ordinal)
            && e.Message.Contains("200", StringComparison.Ordinal));
        await Assert.That(logEntry).IsNotNull();
        await Assert.That(logEntry!.Message).Contains("ms");
    }

    [Test]
    public async Task Should_LogCorrelationIdInScope_When_RequestMade()
    {
        // Arrange
        using var logSink = new SpyLoggerProvider();
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.WithWebHostBuilder(builder =>
        {
            builder.ConfigureServices(services =>
            {
                services.AddLogging(logging =>
                {
                    logging.AddProvider(logSink);
                });
            });
        }).CreateClient();

        var correlationId = "test-corr-id-456";
        using var request = new HttpRequestMessage(HttpMethod.Get, new Uri("/health", UriKind.Relative));
        request.Headers.Add("X-Correlation-Id", correlationId);

        // Act
        using var response = await client.SendAsync(request);

        // Assert
        var logEntry = logSink.Entries.FirstOrDefault(e =>
            e.Message.Contains("/health", StringComparison.Ordinal)
            && e.Message.Contains("200", StringComparison.Ordinal));
        await Assert.That(logEntry).IsNotNull();
        await Assert.That(logEntry!.Scopes).Contains(correlationId);
    }
}
