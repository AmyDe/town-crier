using Microsoft.Extensions.Logging;

namespace TownCrier.Web.Tests.Observability;

internal sealed record SpyLogEntry(LogLevel LogLevel, string Message, string Scopes);
