using System.Diagnostics;
using Microsoft.AspNetCore.Http;
using Microsoft.Extensions.Logging;
using TownCrier.Web.Observability;

namespace TownCrier.Web.Tests.Observability;

public sealed class ErrorResponseMiddlewareExceptionCaptureTests
{
    [Test]
    public async Task Should_RecordExceptionOnActivity_When_UnhandledExceptionThrown()
    {
        // Arrange
        using var listener = new ActivityListener
        {
            ShouldListenTo = _ => true,
            Sample = (ref ActivityCreationOptions<ActivityContext> _) => ActivitySamplingResult.AllDataAndRecorded,
        };
        ActivitySource.AddActivityListener(listener);

        using var source = new ActivitySource("Test.ErrorMiddleware");
        using var activity = source.StartActivity("TestRequest")!;

        var thrownException = new InvalidOperationException("Something broke");
        var middleware = new ErrorResponseMiddleware(
            next: _ => throw thrownException,
            logger: new FakeLogger<ErrorResponseMiddleware>());

        var context = new DefaultHttpContext();

        // Act
        await middleware.InvokeAsync(context);

        // Assert -- the exception should be recorded on the activity
        var exceptionEvent = activity.Events.FirstOrDefault(e => e.Name == "exception");
        await Assert.That(exceptionEvent.Name).IsEqualTo("exception");

        var exceptionType = exceptionEvent.Tags.FirstOrDefault(t => t.Key == "exception.type").Value as string;
        await Assert.That(exceptionType).IsEqualTo("System.InvalidOperationException");

        var exceptionMessage = exceptionEvent.Tags.FirstOrDefault(t => t.Key == "exception.message").Value as string;
        await Assert.That(exceptionMessage).IsEqualTo("Something broke");

        await Assert.That(activity.Status).IsEqualTo(ActivityStatusCode.Error);
        await Assert.That(activity.StatusDescription).IsEqualTo("Something broke");
    }

    [Test]
    public async Task Should_LogExceptionAtErrorLevel_When_UnhandledExceptionThrown()
    {
        // Arrange
        var logger = new SpyLogger<ErrorResponseMiddleware>();
        var thrownException = new InvalidOperationException("Database connection lost");
        var middleware = new ErrorResponseMiddleware(
            next: _ => throw thrownException,
            logger: logger);

        var context = new DefaultHttpContext();

        // Act
        await middleware.InvokeAsync(context);

        // Assert -- the exception must be logged at Error level with the exception object.
        // The OTel logging pipeline reads LogRecord.Exception to populate the exceptions table.
        // The formatted message comes from [LoggerMessage] template; the exception is separate.
        await Assert.That(logger.LogEntries).HasCount().EqualTo(1);

        var entry = logger.LogEntries[0];
        await Assert.That(entry.LogLevel).IsEqualTo(LogLevel.Error);
        await Assert.That(entry.Exception).IsEqualTo(thrownException);
        await Assert.That(entry.Exception!.Message).IsEqualTo("Database connection lost");
        await Assert.That(entry.Message).Contains("Unhandled exception on");
    }

    internal sealed record LogEntry(LogLevel LogLevel, string Message, Exception? Exception);

    private sealed class FakeLogger<T> : ILogger<T>
    {
        public IDisposable? BeginScope<TState>(TState state)
            where TState : notnull => null;

        public bool IsEnabled(LogLevel logLevel) => true;

        public void Log<TState>(LogLevel logLevel, EventId eventId, TState state, Exception? exception, Func<TState, Exception?, string> formatter)
        {
            // No-op -- just need a valid logger
        }
    }

    private sealed class SpyLogger<T> : ILogger<T>
    {
        public List<LogEntry> LogEntries { get; } = [];

        public IDisposable? BeginScope<TState>(TState state)
            where TState : notnull => null;

        public bool IsEnabled(LogLevel logLevel) => true;

        public void Log<TState>(LogLevel logLevel, EventId eventId, TState state, Exception? exception, Func<TState, Exception?, string> formatter)
        {
            this.LogEntries.Add(new LogEntry(logLevel, formatter(state, exception), exception));
        }
    }
}
