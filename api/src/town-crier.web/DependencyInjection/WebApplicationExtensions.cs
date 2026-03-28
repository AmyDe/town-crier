using TownCrier.Web.Endpoints;
using TownCrier.Web.Observability;
using TownCrier.Web.RateLimiting;

namespace TownCrier.Web.DependencyInjection;

internal static class WebApplicationExtensions
{
    public static WebApplication UseMiddlewarePipeline(this WebApplication app)
    {
        app.UseCors();
        app.UseMiddleware<CorrelationIdMiddleware>();
        app.UseMiddleware<ErrorResponseMiddleware>();
        app.UseMiddleware<RequestLoggingMiddleware>();
        app.UseAuthentication();
        app.UseAuthorization();
        app.UseMiddleware<RateLimitMiddleware>();

        return app;
    }

    public static WebApplication MapAllEndpoints(this WebApplication app)
    {
        app.MapHealthEndpoints();
        app.MapV1Endpoints();
        app.MapApiEndpoints();

        return app;
    }
}
