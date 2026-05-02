using System.Security.Claims;
using TownCrier.Application.SavedApplications;
using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Web.Endpoints;

internal static class SavedApplicationEndpoints
{
    public static void MapSavedApplicationEndpoints(this RouteGroupBuilder group)
    {
        group.MapPut("/me/saved-applications/{**applicationUid}", async (
            ClaimsPrincipal user,
            string applicationUid,
            SaveApplicationRequest request,
            SaveApplicationCommandHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;
            if (request is null || string.IsNullOrWhiteSpace(request.Uid) || request.Uid != applicationUid)
            {
                return Results.Json(
                    new ApiErrorResponse("Body uid must match path uid."),
                    AppJsonSerializerContext.Default.ApiErrorResponse,
                    statusCode: 400);
            }

            var application = new PlanningApplication(
                name: request.Name,
                uid: request.Uid,
                areaName: request.AreaName,
                areaId: request.AreaId,
                address: request.Address,
                postcode: request.Postcode,
                description: request.Description,
                appType: request.AppType,
                appState: request.AppState,
                appSize: request.AppSize,
                startDate: request.StartDate,
                decidedDate: request.DecidedDate,
                consultedDate: request.ConsultedDate,
                longitude: request.Longitude,
                latitude: request.Latitude,
                url: request.Url,
                link: request.Link,
                lastDifferent: request.LastDifferent);

            await handler.HandleAsync(new SaveApplicationCommand(userId, application), ct).ConfigureAwait(false);
            return Results.NoContent();
        });

        group.MapDelete("/me/saved-applications/{**applicationUid}", async (
            ClaimsPrincipal user,
            string applicationUid,
            RemoveSavedApplicationCommandHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;
            await handler.HandleAsync(new RemoveSavedApplicationCommand(userId, applicationUid), ct).ConfigureAwait(false);
            return Results.NoContent();
        });

        group.MapGet("/me/saved-applications", async (
            ClaimsPrincipal user,
            GetSavedApplicationsQueryHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;
            var result = await handler.HandleAsync(new GetSavedApplicationsQuery(userId), ct).ConfigureAwait(false);
            return Results.Ok(result);
        });
    }
}
