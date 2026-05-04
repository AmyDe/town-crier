using System.Security.Claims;
using TownCrier.Application.NotificationState;

namespace TownCrier.Web.Endpoints;

internal static class NotificationStateEndpoints
{
    public static void MapNotificationStateEndpoints(this RouteGroupBuilder group)
    {
        group.MapGet("/me/notification-state", async (
            ClaimsPrincipal user,
            GetNotificationStateQueryHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;
            var result = await handler
                .HandleAsync(new GetNotificationStateQuery(userId), ct)
                .ConfigureAwait(false);
            return Results.Ok(result);
        });

        group.MapPost("/me/notification-state/mark-all-read", async (
            ClaimsPrincipal user,
            MarkAllNotificationsReadCommandHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;
            await handler
                .HandleAsync(new MarkAllNotificationsReadCommand(userId), ct)
                .ConfigureAwait(false);
            return Results.NoContent();
        });

        group.MapPost("/me/notification-state/advance", async (
            ClaimsPrincipal user,
            AdvanceNotificationStateRequest request,
            AdvanceNotificationStateCommandHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;
            var command = new AdvanceNotificationStateCommand(userId, request.AsOf);
            await handler.HandleAsync(command, ct).ConfigureAwait(false);
            return Results.NoContent();
        });
    }
}
