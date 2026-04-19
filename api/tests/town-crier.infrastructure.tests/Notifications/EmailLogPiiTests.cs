using Microsoft.Extensions.Logging;
using TownCrier.Infrastructure.Notifications;

namespace TownCrier.Infrastructure.Tests.Notifications;

public sealed class EmailLogPiiTests
{
    private const string SensitiveEmail = "sensitive.user@example.com";
    private const string UserId = "auth0|abc123";

    [Test]
    public async Task Should_NotContainEmailInStructuredState_When_DigestSendFailedLogged()
    {
        // Arrange
        var logger = new SpyLogger<AcsEmailSender>();
        var exception = new InvalidOperationException("boom");

        // Act
        EmailLog.DigestSendFailed(logger, UserId, exception);

        // Assert
        await Assert.That(logger.Entries).HasCount().EqualTo(1);
        var entry = logger.Entries[0];

        await Assert.That(entry.Message).DoesNotContain(SensitiveEmail);
        await Assert.That(entry.Message).DoesNotContain("@");
        foreach (var kvp in entry.State)
        {
            var text = kvp.Value?.ToString() ?? string.Empty;
            await Assert.That(text).DoesNotContain(SensitiveEmail);
            await Assert.That(text).DoesNotContain("@");
        }
    }

    [Test]
    public async Task Should_NotContainEmailInStructuredState_When_NotificationSendFailedLogged()
    {
        // Arrange
        var logger = new SpyLogger<AcsEmailSender>();
        var exception = new InvalidOperationException("boom");

        // Act
        EmailLog.NotificationSendFailed(logger, UserId, exception);

        // Assert
        await Assert.That(logger.Entries).HasCount().EqualTo(1);
        var entry = logger.Entries[0];

        await Assert.That(entry.Message).DoesNotContain(SensitiveEmail);
        await Assert.That(entry.Message).DoesNotContain("@");
        foreach (var kvp in entry.State)
        {
            var text = kvp.Value?.ToString() ?? string.Empty;
            await Assert.That(text).DoesNotContain(SensitiveEmail);
            await Assert.That(text).DoesNotContain("@");
        }
    }

    [Test]
    public async Task Should_IncludeUserIdInStructuredState_When_DigestSendFailedLogged()
    {
        // Arrange
        var logger = new SpyLogger<AcsEmailSender>();

        // Act
        EmailLog.DigestSendFailed(logger, UserId, new InvalidOperationException("boom"));

        // Assert
        var entry = logger.Entries[0];
        var hasUserId = entry.State.Any(kvp =>
            string.Equals(kvp.Key, "UserId", StringComparison.Ordinal)
            && string.Equals(kvp.Value?.ToString(), UserId, StringComparison.Ordinal));
        await Assert.That(hasUserId).IsTrue();
    }

    [Test]
    public async Task Should_IncludeUserIdInStructuredState_When_NotificationSendFailedLogged()
    {
        // Arrange
        var logger = new SpyLogger<AcsEmailSender>();

        // Act
        EmailLog.NotificationSendFailed(logger, UserId, new InvalidOperationException("boom"));

        // Assert
        var entry = logger.Entries[0];
        var hasUserId = entry.State.Any(kvp =>
            string.Equals(kvp.Key, "UserId", StringComparison.Ordinal)
            && string.Equals(kvp.Value?.ToString(), UserId, StringComparison.Ordinal));
        await Assert.That(hasUserId).IsTrue();
    }

    internal sealed record SpyLogEntry(
        LogLevel LogLevel,
        string Message,
        IReadOnlyList<KeyValuePair<string, object?>> State,
        Exception? Exception);

    private sealed class SpyLogger<T> : ILogger<T>
    {
        public List<SpyLogEntry> Entries { get; } = [];

        public IDisposable? BeginScope<TState>(TState state)
            where TState : notnull => null;

        public bool IsEnabled(LogLevel logLevel) => true;

        public void Log<TState>(
            LogLevel logLevel,
            EventId eventId,
            TState state,
            Exception? exception,
            Func<TState, Exception?, string> formatter)
        {
            // Snapshot the state because LoggerMessage generator uses a pooled, thread-local
            // state buffer that gets cleared after the Log call returns. We must materialize
            // the KeyValuePair entries before returning.
            var kvps = new List<KeyValuePair<string, object?>>();
            if (state is IReadOnlyList<KeyValuePair<string, object?>> list)
            {
                for (var i = 0; i < list.Count; i++)
                {
                    kvps.Add(list[i]);
                }
            }
            else if (state is IEnumerable<KeyValuePair<string, object?>> enumerable)
            {
                foreach (var kvp in enumerable)
                {
                    kvps.Add(kvp);
                }
            }

            this.Entries.Add(new SpyLogEntry(logLevel, formatter(state, exception), kvps, exception));
        }
    }
}
