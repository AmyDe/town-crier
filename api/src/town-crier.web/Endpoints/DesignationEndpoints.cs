using TownCrier.Application.Designations;

namespace TownCrier.Web.Endpoints;

internal static class DesignationEndpoints
{
    public static RouteGroupBuilder MapDesignationEndpoints(this RouteGroupBuilder group)
    {
        group.MapGet("/designations", async (
            double latitude,
            double longitude,
            GetDesignationContextQueryHandler handler,
            CancellationToken ct) =>
        {
            var query = new GetDesignationContextQuery(latitude, longitude);
            var result = await handler.HandleAsync(query, ct).ConfigureAwait(false);
            return Results.Ok(result);
        });

        return group;
    }
}
