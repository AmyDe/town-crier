using TownCrier.Application.Health;

namespace TownCrier.Web.Endpoints;

internal static class HealthEndpoints
{
    public static void MapHealthEndpoints(this IEndpointRouteBuilder app)
    {
        app.MapGet("/health", () => CheckHealthQueryHandler.HandleAsync(new CheckHealthQuery(), CancellationToken.None))
            .AllowAnonymous();
    }
}
