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

        // Partitioned point read: authorityCode + name uniquely identifies the Cosmos
        // document (id = name, pk = authorityCode). Replaces the old cross-partition
        // uid scan. No PlanIt fallback — see GH#395 Invariant 1.
        group.MapGet("/applications/{authorityCode}/{**name}", async (
            ClaimsPrincipal user,
            string authorityCode,
            string name,
            GetApplicationByAuthorityAndNameQueryHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub");
            var result = await handler.HandleAsync(
                new GetApplicationByAuthorityAndNameQuery(authorityCode, name, userId),
                ct).ConfigureAwait(false);
            return result is null ? Results.NotFound() : Results.Ok(result);
        });
    }
}
