using Microsoft.Extensions.Logging;

namespace TownCrier.Worker;

internal static partial class WorkerLog
{
    [LoggerMessage(Level = LogLevel.Information, Message = "Starting poll cycle")]
    internal static partial void PollCycleStarting(ILogger logger);

    [LoggerMessage(
        Level = LogLevel.Information,
        Message = "Poll cycle completed: {ApplicationCount} applications from {AuthoritiesPolled} authorities")]
    internal static partial void PollCycleCompleted(ILogger logger, int applicationCount, int authoritiesPolled);

    [LoggerMessage(Level = LogLevel.Error, Message = "Poll cycle failed")]
    internal static partial void PollCycleFailed(ILogger logger, Exception exception);

    [LoggerMessage(Level = LogLevel.Information, Message = "Starting weekly digest generation")]
    internal static partial void DigestCycleStarting(ILogger logger);

    [LoggerMessage(Level = LogLevel.Information, Message = "Weekly digest generation completed")]
    internal static partial void DigestCycleCompleted(ILogger logger);

    [LoggerMessage(Level = LogLevel.Error, Message = "Weekly digest generation failed")]
    internal static partial void DigestCycleFailed(ILogger logger, Exception exception);

    [LoggerMessage(Level = LogLevel.Critical, Message = "Unknown WORKER_MODE '{WorkerMode}'. Valid values: poll, digest")]
    internal static partial void UnknownWorkerMode(ILogger logger, string workerMode);
}
