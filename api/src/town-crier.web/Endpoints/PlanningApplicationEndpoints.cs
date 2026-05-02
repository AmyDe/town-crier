using System.Security.Claims;
using TownCrier.Application.PlanningApplications;

namespace TownCrier.Web.Endpoints;

internal static class PlanningApplicationEndpoints
{
    public static void MapPlanningApplicationEndpoints(this RouteGroupBuilder group)
    {
        group.MapGet("/me/application-authorities", async (
            ClaimsPrincipal user,
            GetUserApplicationAuthoritiesQueryHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;
            var result = await handler.HandleAsync(
                new GetUserApplicationAuthoritiesQuery(userId), ct).ConfigureAwait(false);
            return Results.Ok(result);
        });

        group.MapGet("/applications/{**uid}", async (
            ClaimsPrincipal user,
            string uid,
            GetApplicationByUidQueryHandler handler,
            CancellationToken ct) =>
        {
            // userId enables refresh-on-tap: opening a saved item silently
            // upserts the latest snapshot back into the saved row so the
            // saved-list self-heals over time. See bd tc-udby.
            var userId = user.FindFirstValue("sub");
            var result = await handler.HandleAsync(
                new GetApplicationByUidQuery(uid, userId), ct).ConfigureAwait(false);
            return result is null ? Results.NotFound() : Results.Ok(result);
        });
    }
}
