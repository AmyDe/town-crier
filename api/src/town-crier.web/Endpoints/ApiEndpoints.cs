using System.Security.Claims;

namespace TownCrier.Web.Endpoints;

internal static class ApiEndpoints
{
    public static void MapApiEndpoints(this IEndpointRouteBuilder app)
    {
        var api = app.MapGroup("/api");

        api.MapGet("/me", (ClaimsPrincipal user) =>
        {
            var userId = user.FindFirstValue("sub")!;
            return Results.Ok(new UserIdResponse(userId));
        });
    }
}
