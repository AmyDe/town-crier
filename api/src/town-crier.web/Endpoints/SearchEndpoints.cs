using System.Security.Claims;
using TownCrier.Application.Search;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.Entitlements;

namespace TownCrier.Web.Endpoints;

internal static class SearchEndpoints
{
    public static void MapSearchEndpoints(this RouteGroupBuilder group)
    {
        group.MapGet("/search", async (
            ClaimsPrincipal user,
            string q,
            int authorityId,
            int page,
            SearchPlanningApplicationsQueryHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;
            var query = new SearchPlanningApplicationsQuery(userId, q, authorityId, page);

            try
            {
                var result = await handler.HandleAsync(query, ct).ConfigureAwait(false);
                return Results.Ok(result);
            }
            catch (UserProfileNotFoundException)
            {
                return Results.NotFound();
            }
        })
        .AddEndpointFilter<EntitlementEndpointFilter>()
        .WithMetadata(new RequiresEntitlementAttribute(Entitlement.SearchApplications));
    }
}
