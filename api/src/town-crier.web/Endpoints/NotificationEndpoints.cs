using System.Security.Claims;
using TownCrier.Application.Notifications;

namespace TownCrier.Web.Endpoints;

internal static class NotificationEndpoints
{
    public static void MapNotificationEndpoints(this RouteGroupBuilder group)
    {
        group.MapGet("/notifications", async (
            ClaimsPrincipal user,
            int? page,
            int? pageSize,
            GetNotificationsQueryHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;
            var query = new GetNotificationsQuery(userId, page ?? 1, pageSize ?? 20);
            var result = await handler.HandleAsync(query, ct).ConfigureAwait(false);
            return Results.Ok(result);
        });
    }
}
