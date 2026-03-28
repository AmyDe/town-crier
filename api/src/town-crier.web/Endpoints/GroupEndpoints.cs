using System.Security.Claims;
using TownCrier.Application.Groups;
using TownCrier.Domain.Groups;

namespace TownCrier.Web.Endpoints;

internal static class GroupEndpoints
{
    public static void MapGroupEndpoints(this RouteGroupBuilder group)
    {
        group.MapPost("/groups", async (
            ClaimsPrincipal user,
            CreateGroupCommand command,
            CreateGroupCommandHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;
            var fullCommand = new CreateGroupCommand(
                userId,
                command.GroupId,
                command.Name,
                command.Latitude,
                command.Longitude,
                command.RadiusMetres,
                command.AuthorityId);
            var result = await handler.HandleAsync(fullCommand, ct).ConfigureAwait(false);
            return Results.Created($"/v1/groups/{result.GroupId}", result);
        });

        group.MapGet("/groups", async (
            ClaimsPrincipal user,
            GetUserGroupsQueryHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;
            var result = await handler.HandleAsync(new GetUserGroupsQuery(userId), ct).ConfigureAwait(false);
            return Results.Ok(result);
        });

        group.MapGet("/groups/{groupId}", async (
            ClaimsPrincipal user,
            string groupId,
            GetGroupQueryHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;

            try
            {
                var result = await handler.HandleAsync(
                    new GetGroupQuery(userId, groupId), ct).ConfigureAwait(false);
                return Results.Ok(result);
            }
            catch (GroupNotFoundException)
            {
                return Results.NotFound();
            }
        });

        group.MapDelete("/groups/{groupId}", async (
            ClaimsPrincipal user,
            string groupId,
            DeleteGroupCommandHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;

            try
            {
                await handler.HandleAsync(
                    new DeleteGroupCommand(userId, groupId), ct).ConfigureAwait(false);
                return Results.NoContent();
            }
            catch (GroupNotFoundException)
            {
                return Results.NotFound();
            }
            catch (UnauthorizedGroupOperationException)
            {
                return Results.Json(
                    new ApiErrorResponse("Only the group owner can delete the group."),
                    AppJsonSerializerContext.Default.ApiErrorResponse,
                    statusCode: 403);
            }
        });
    }
}
