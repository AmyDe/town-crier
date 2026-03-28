using TownCrier.Application.PlanningApplications;

namespace TownCrier.Web.Endpoints;

internal static class PlanningApplicationEndpoints
{
    public static void MapPlanningApplicationEndpoints(this RouteGroupBuilder group)
    {
        group.MapGet("/applications", async (
            int authorityId,
            GetApplicationsByAuthorityQueryHandler handler,
            CancellationToken ct) =>
        {
            var result = await handler.HandleAsync(
                new GetApplicationsByAuthorityQuery(authorityId), ct).ConfigureAwait(false);
            return Results.Ok(result);
        });

        group.MapGet("/applications/{uid}", async (
            string uid,
            GetApplicationByUidQueryHandler handler,
            CancellationToken ct) =>
        {
            var result = await handler.HandleAsync(
                new GetApplicationByUidQuery(uid), ct).ConfigureAwait(false);
            return result is null ? Results.NotFound() : Results.Ok(result);
        });
    }
}
