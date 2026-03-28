using TownCrier.Application.Health;

namespace TownCrier.Web.Endpoints;

internal static class V1HealthEndpoints
{
    public static RouteGroupBuilder MapHealthEndpoints(this RouteGroupBuilder group)
    {
        group.MapGet("/health", () => CheckHealthQueryHandler.HandleAsync(
            new CheckHealthQuery(), CancellationToken.None))
            .AllowAnonymous();

        return group;
    }
}
