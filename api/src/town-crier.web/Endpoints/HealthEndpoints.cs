using TownCrier.Application.Health;

namespace TownCrier.Web.Endpoints;

internal static class HealthEndpoints
{
    public static WebApplication MapHealthEndpoints(this WebApplication app)
    {
        app.MapGet("/health", () => CheckHealthQueryHandler.HandleAsync(
            new CheckHealthQuery(), CancellationToken.None))
            .AllowAnonymous();

        return app;
    }
}
