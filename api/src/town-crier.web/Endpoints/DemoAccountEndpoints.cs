using TownCrier.Application.DemoAccount;

namespace TownCrier.Web.Endpoints;

internal static class DemoAccountEndpoints
{
    public static void MapDemoAccountEndpoints(this RouteGroupBuilder group)
    {
        group.MapGet("/demo-account", async (
            GetDemoAccountQueryHandler handler,
            CancellationToken ct) =>
        {
            var result = await handler.HandleAsync(new GetDemoAccountQuery(), ct).ConfigureAwait(false);
            return Results.Ok(result);
        }).AllowAnonymous();
    }
}
