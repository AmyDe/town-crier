using TownCrier.Application.VersionConfig;

namespace TownCrier.Web.Endpoints;

internal static class VersionConfigEndpoints
{
    public static RouteGroupBuilder MapVersionConfigEndpoints(this RouteGroupBuilder group)
    {
        group.MapGet("/version-config", () => GetVersionConfigQueryHandler.HandleAsync(
            new GetVersionConfigQuery(), CancellationToken.None))
            .AllowAnonymous();

        return group;
    }
}
