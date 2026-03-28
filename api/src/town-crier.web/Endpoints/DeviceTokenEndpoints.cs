using System.Security.Claims;
using TownCrier.Application.DeviceRegistrations;

namespace TownCrier.Web.Endpoints;

internal static class DeviceTokenEndpoints
{
    public static void MapDeviceTokenEndpoints(this RouteGroupBuilder group)
    {
        group.MapPut("/me/device-token", async (
            ClaimsPrincipal user,
            RegisterDeviceTokenRequest request,
            RegisterDeviceTokenCommandHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;
            var command = new RegisterDeviceTokenCommand(userId, request.Token, request.Platform);
            await handler.HandleAsync(command, ct).ConfigureAwait(false);
            return Results.NoContent();
        });

        group.MapDelete("/me/device-token/{token}", async (
            ClaimsPrincipal user,
            string token,
            RemoveInvalidDeviceTokenCommandHandler handler,
            CancellationToken ct) =>
        {
            var command = new RemoveInvalidDeviceTokenCommand(token);
            await handler.HandleAsync(command, ct).ConfigureAwait(false);
            return Results.NoContent();
        });
    }
}
