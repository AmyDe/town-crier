using System.Collections.Concurrent;
using Microsoft.Extensions.Logging;

namespace TownCrier.Web.Tests.Observability;

internal sealed class SpyLoggerProvider : ILoggerProvider
{
    private readonly ConcurrentBag<SpyLogEntry> entries = [];

    internal IReadOnlyCollection<SpyLogEntry> Entries => this.entries.ToArray();

    public ILogger CreateLogger(string categoryName)
    {
        return new SpyLogger(this.entries);
    }

    public void Dispose()
    {
        // No resources to dispose
    }

    private sealed class SpyLogger(ConcurrentBag<SpyLogEntry> entries) : ILogger
    {
        private readonly List<string> currentScopes = [];

        public IDisposable? BeginScope<TState>(TState state)
            where TState : notnull
        {
            var scopeText = state.ToString() ?? string.Empty;
            this.currentScopes.Add(scopeText);
            return new ScopeDisposable(this.currentScopes, scopeText);
        }

        public bool IsEnabled(LogLevel logLevel)
        {
            return true;
        }

        public void Log<TState>(
            LogLevel logLevel,
            EventId eventId,
            TState state,
            Exception? exception,
            Func<TState, Exception?, string> formatter)
        {
            var message = formatter(state, exception);
            var scopeSnapshot = string.Join(", ", this.currentScopes);
            entries.Add(new SpyLogEntry(logLevel, message, scopeSnapshot));
        }

        private sealed class ScopeDisposable(List<string> scopes, string scope) : IDisposable
        {
            public void Dispose()
            {
                scopes.Remove(scope);
            }
        }
    }
}
