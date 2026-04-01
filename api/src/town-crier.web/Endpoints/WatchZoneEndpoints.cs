using System.Security.Claims;
using TownCrier.Application.UserProfiles;
using TownCrier.Application.WatchZones;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Web.Endpoints;

internal static class WatchZoneEndpoints
{
    public static void MapWatchZoneEndpoints(this RouteGroupBuilder group)
    {
        group.MapPost("/me/watch-zones", async (
            ClaimsPrincipal user,
            CreateWatchZoneRequest request,
            CreateWatchZoneCommandHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;
            var zoneId = Guid.NewGuid().ToString();
            var command = new CreateWatchZoneCommand(
                userId,
                zoneId,
                request.Name,
                request.Latitude,
                request.Longitude,
                request.RadiusMetres,
                request.AuthorityId);
            var result = await handler.HandleAsync(command, ct).ConfigureAwait(false);
            return Results.Created($"/v1/me/watch-zones/{zoneId}", result);
        });

        group.MapGet("/me/watch-zones", async (
            ClaimsPrincipal user,
            ListWatchZonesQueryHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;
            var result = await handler.HandleAsync(new ListWatchZonesQuery(userId), ct).ConfigureAwait(false);
            return Results.Ok(result);
        });

        group.MapDelete("/me/watch-zones/{zoneId}", async (
            ClaimsPrincipal user,
            string zoneId,
            DeleteWatchZoneCommandHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;

            try
            {
                await handler.HandleAsync(
                    new DeleteWatchZoneCommand(userId, zoneId), ct).ConfigureAwait(false);
                return Results.NoContent();
            }
            catch (WatchZoneNotFoundException)
            {
                return Results.NotFound();
            }
        });

        group.MapGet("/me/watch-zones/{zoneId}/preferences", async (
            ClaimsPrincipal user,
            string zoneId,
            GetZonePreferencesQueryHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;

            try
            {
                var result = await handler.HandleAsync(
                    new GetZonePreferencesQuery(userId, zoneId), ct).ConfigureAwait(false);
                return Results.Ok(result);
            }
            catch (UserProfileNotFoundException)
            {
                return Results.NotFound();
            }
        });

        group.MapPut("/me/watch-zones/{zoneId}/preferences", async (
            ClaimsPrincipal user,
            string zoneId,
            UpdateZonePreferencesCommand command,
            UpdateZonePreferencesCommandHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;
            var fullCommand = new UpdateZonePreferencesCommand(
                userId,
                zoneId,
                command.NewApplications,
                command.StatusChanges,
                command.DecisionUpdates);

            try
            {
                var result = await handler.HandleAsync(fullCommand, ct).ConfigureAwait(false);
                return Results.Ok(result);
            }
            catch (UserProfileNotFoundException)
            {
                return Results.NotFound();
            }
            catch (InsufficientTierException)
            {
                return Results.Json(
                    new ApiErrorResponse("This feature requires a Pro subscription."),
                    AppJsonSerializerContext.Default.ApiErrorResponse,
                    statusCode: 403);
            }
        });
    }
}
