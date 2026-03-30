using System.Text.Json;

namespace TownCrier.Web.Observability;

internal sealed class ErrorResponseMiddleware(RequestDelegate next)
{
    private const string CorrelationIdHeader = "X-Correlation-Id";

    public async Task InvokeAsync(HttpContext context)
    {
        try
        {
            await next(context).ConfigureAwait(false);
        }
#pragma warning disable CA1031 // Global error handler must catch all exceptions
        catch (Exception)
#pragma warning restore CA1031
        {
            if (!context.Response.HasStarted)
            {
                context.Response.StatusCode = 500;
            }
        }

        if (context.Response.StatusCode >= 400
            && !context.Response.HasStarted
            && context.Response.ContentLength is null or 0)
        {
            var correlationId = context.Request.Headers[CorrelationIdHeader].FirstOrDefault()
                ?? context.Response.Headers[CorrelationIdHeader].FirstOrDefault()
                ?? string.Empty;

            var errorBody = new ErrorResponse(
                context.Response.StatusCode,
                GetReasonPhrase(context.Response.StatusCode),
                correlationId);

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
}
