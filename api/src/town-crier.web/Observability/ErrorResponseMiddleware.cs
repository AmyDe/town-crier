using System.Text.Json;

namespace TownCrier.Web.Observability;

internal sealed partial class ErrorResponseMiddleware(RequestDelegate next, ILogger<ErrorResponseMiddleware> logger)
{
    public async Task InvokeAsync(HttpContext context)
    {
        try
        {
            await next(context).ConfigureAwait(false);
        }
#pragma warning disable CA1031 // Global error handler must catch all exceptions
        catch (Exception ex)
#pragma warning restore CA1031
        {
            LogUnhandledException(logger, ex, context.Request.Method, context.Request.Path.Value ?? "/");
            context.Items["ErrorDetail"] = ex.Message;
            if (!context.Response.HasStarted)
            {
                context.Response.StatusCode = 500;
            }
        }

        if (context.Response.StatusCode >= 400
            && !context.Response.HasStarted
            && context.Response.ContentLength is null or 0)
        {
            var detail = context.Items.TryGetValue("ErrorDetail", out var detailObj)
                ? detailObj as string
                : null;

            var errorBody = new ErrorResponse(
                context.Response.StatusCode,
                GetReasonPhrase(context.Response.StatusCode),
                detail);

            context.Response.ContentType = "application/json";
            await JsonSerializer.SerializeAsync(
                context.Response.Body,
                errorBody,
                ObservabilityJsonSerializerContext.Default.ErrorResponse,
                context.RequestAborted).ConfigureAwait(false);
        }
    }

    private static string GetReasonPhrase(int statusCode)
    {
        return statusCode switch
        {
            400 => "Bad Request",
            401 => "Unauthorized",
            403 => "Forbidden",
            404 => "Not Found",
            405 => "Method Not Allowed",
            500 => "Internal Server Error",
            _ => "Error",
        };
    }

    [LoggerMessage(Level = LogLevel.Error, Message = "Unhandled exception on {Method} {Path}")]
    private static partial void LogUnhandledException(ILogger logger, Exception exception, string method, string path);
}
