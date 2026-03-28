using System.Security.Claims;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Web.Endpoints;

internal static class UserProfileEndpoints
{
    public static RouteGroupBuilder MapUserProfileEndpoints(this RouteGroupBuilder group)
    {
        group.MapPost("/me", async (
            ClaimsPrincipal user,
            CreateUserProfileCommandHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;
            var result = await handler.HandleAsync(
                new CreateUserProfileCommand(userId), ct).ConfigureAwait(false);
            return Results.Ok(result);
        });

        group.MapGet("/me", async (
            ClaimsPrincipal user,
            GetUserProfileQueryHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;
            var result = await handler.HandleAsync(
                new GetUserProfileQuery(userId), ct).ConfigureAwait(false);
            return result is null ? Results.NotFound() : Results.Ok(result);
        });

        group.MapPatch("/me", async (
            ClaimsPrincipal user,
            UpdateUserProfileCommand command,
            UpdateUserProfileCommandHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;
            var profileCommand = new UpdateUserProfileCommand(
                userId, command.Postcode, command.PushEnabled);

            try
            {
                var result = await handler.HandleAsync(profileCommand, ct).ConfigureAwait(false);
                return Results.Ok(result);
            }
            catch (UserProfileNotFoundException)
            {
                return Results.NotFound();
            }
        });

        group.MapGet("/me/data", async (
            ClaimsPrincipal user,
            ExportUserDataQueryHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;
            var result = await handler.HandleAsync(
                new ExportUserDataQuery(userId), ct).ConfigureAwait(false);
            return result is null ? Results.NotFound() : Results.Ok(result);
        });

        group.MapDelete("/me", async (
            ClaimsPrincipal user,
            DeleteUserProfileCommandHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;

            try
            {
                await handler.HandleAsync(
                    new DeleteUserProfileCommand(userId), ct).ConfigureAwait(false);
                return Results.NoContent();
            }
            catch (UserProfileNotFoundException)
            {
                return Results.NotFound();
            }
        });

        return group;
    }
}
