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

    [LoggerMessage(Level = LogLevel.Information, Message = "Starting hourly digest generation")]
    internal static partial void HourlyDigestCycleStarting(ILogger logger);

    [LoggerMessage(Level = LogLevel.Information, Message = "Hourly digest generation completed")]
    internal static partial void HourlyDigestCycleCompleted(ILogger logger);

    [LoggerMessage(Level = LogLevel.Error, Message = "Hourly digest generation failed")]
    internal static partial void HourlyDigestCycleFailed(ILogger logger, Exception exception);

    [LoggerMessage(Level = LogLevel.Information, Message = "Starting dormant account cleanup")]
    internal static partial void DormantCleanupStarting(ILogger logger);

    [LoggerMessage(
        Level = LogLevel.Information,
        Message = "Dormant account cleanup completed: {DeletedCount} profiles deleted")]
    internal static partial void DormantCleanupCompleted(ILogger logger, int deletedCount);

    [LoggerMessage(Level = LogLevel.Error, Message = "Dormant account cleanup failed")]
    internal static partial void DormantCleanupFailed(ILogger logger, Exception exception);

    [LoggerMessage(Level = LogLevel.Critical, Message = "Unknown WORKER_MODE '{WorkerMode}'. Valid values: poll-sb, poll-bootstrap, digest, hourly-digest, dormant-cleanup")]
    internal static partial void UnknownWorkerMode(ILogger logger, string workerMode);

    [LoggerMessage(Level = LogLevel.Critical, Message = "Polling handler budget must be set for poll-sb mode to bound lease TTL. Aborting")]
    internal static partial void HandlerBudgetMissingInPollSbMode(ILogger logger);
}
