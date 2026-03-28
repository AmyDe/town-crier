using System.Security.Claims;
using TownCrier.Application.SavedApplications;

namespace TownCrier.Web.Endpoints;

internal static class SavedApplicationEndpoints
{
    public static RouteGroupBuilder MapSavedApplicationEndpoints(this RouteGroupBuilder group)
    {
        group.MapPut("/me/saved-applications/{applicationUid}", async (
            ClaimsPrincipal user,
            string applicationUid,
            SaveApplicationCommandHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;
            await handler.HandleAsync(
                new SaveApplicationCommand(userId, applicationUid), ct).ConfigureAwait(false);
            return Results.NoContent();
        });

        group.MapDelete("/me/saved-applications/{applicationUid}", async (
            ClaimsPrincipal user,
            string applicationUid,
            RemoveSavedApplicationCommandHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;
            await handler.HandleAsync(
                new RemoveSavedApplicationCommand(userId, applicationUid), ct).ConfigureAwait(false);
            return Results.NoContent();
        });

        group.MapGet("/me/saved-applications", async (
            ClaimsPrincipal user,
            GetSavedApplicationsQueryHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;
            var result = await handler.HandleAsync(
                new GetSavedApplicationsQuery(userId), ct).ConfigureAwait(false);
            return Results.Ok(result);
        });

        return group;
    }
}
