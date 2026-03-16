using System.Diagnostics;

namespace TownCrier.Web.Observability;

internal sealed partial class RequestLoggingMiddleware(RequestDelegate next, ILogger<RequestLoggingMiddleware> logger)
{
    private const string CorrelationIdHeader = "X-Correlation-Id";

    private static readonly Func<ILogger, string, IDisposable?> BeginCorrelationScope =
        LoggerMessage.DefineScope<string>("CorrelationId: {CorrelationId}");

    public async Task InvokeAsync(HttpContext context)
    {
        var correlationId = context.Request.Headers[CorrelationIdHeader].FirstOrDefault()
            ?? Guid.NewGuid().ToString();

        using (BeginCorrelationScope(logger, correlationId))
        {
            var stopwatch = Stopwatch.StartNew();

            await next(context).ConfigureAwait(false);

            stopwatch.Stop();

            LogRequestCompleted(
                logger,
                context.Request.Method,
                context.Request.Path,
                context.Response.StatusCode,
                stopwatch.ElapsedMilliseconds);
        }
    }

    [LoggerMessage(Level = LogLevel.Information, Message = "HTTP {Method} {Path} responded {StatusCode} in {ElapsedMs}ms")]
    private static partial void LogRequestCompleted(
        ILogger logger,
        string method,
        string path,
        int statusCode,
        long elapsedMs);
}
