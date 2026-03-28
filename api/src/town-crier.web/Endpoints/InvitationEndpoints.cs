using System.Security.Claims;
using TownCrier.Application.Groups;
using TownCrier.Domain.Groups;

namespace TownCrier.Web.Endpoints;

internal static class InvitationEndpoints
{
    public static void MapInvitationEndpoints(this RouteGroupBuilder group)
    {
        group.MapPost("/groups/{groupId}/invitations", async (
            ClaimsPrincipal user,
            string groupId,
            InviteMemberCommand command,
            InviteMemberCommandHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;
            var fullCommand = new InviteMemberCommand(
                userId,
                groupId,
                command.InvitationId,
                command.InviteeEmail);

            try
            {
                var result = await handler.HandleAsync(fullCommand, ct).ConfigureAwait(false);
                return Results.Created($"/v1/groups/{groupId}/invitations/{result.InvitationId}", result);
            }
            catch (GroupNotFoundException)
            {
                return Results.NotFound();
            }
            catch (UnauthorizedGroupOperationException)
            {
                return Results.Json(
                    new ApiErrorResponse("Only the group owner can invite members."),
                    AppJsonSerializerContext.Default.ApiErrorResponse,
                    statusCode: 403);
            }
        });

        group.MapPost("/invitations/{invitationId}/accept", async (
            ClaimsPrincipal user,
            string invitationId,
            AcceptInvitationCommandHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;

            try
            {
                await handler.HandleAsync(
                    new AcceptInvitationCommand(userId, invitationId), ct).ConfigureAwait(false);
                return Results.NoContent();
            }
            catch (InvalidOperationException ex)
            {
                return Results.BadRequest(new ApiErrorResponse(ex.Message));
            }
            catch (GroupNotFoundException)
            {
                return Results.NotFound();
            }
        });

        group.MapDelete("/groups/{groupId}/members/{memberUserId}", async (
            ClaimsPrincipal user,
            string groupId,
            string memberUserId,
            RemoveGroupMemberCommandHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;

            try
            {
                await handler.HandleAsync(
                    new RemoveGroupMemberCommand(userId, groupId, memberUserId), ct).ConfigureAwait(false);
                return Results.NoContent();
            }
            catch (GroupNotFoundException)
            {
                return Results.NotFound();
            }
            catch (UnauthorizedGroupOperationException)
            {
                return Results.Json(
                    new ApiErrorResponse("Only the group owner can remove members."),
                    AppJsonSerializerContext.Default.ApiErrorResponse,
                    statusCode: 403);
            }
        });
    }
}
