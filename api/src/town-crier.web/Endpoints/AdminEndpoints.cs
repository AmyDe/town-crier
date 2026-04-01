using TownCrier.Application.Admin;
using TownCrier.Application.UserProfiles;

namespace TownCrier.Web.Endpoints;

internal static class AdminEndpoints
{
    public static void MapAdminEndpoints(this RouteGroupBuilder group)
    {
        var admin = group.MapGroup("/admin")
            .AddEndpointFilter<AdminApiKeyFilter>()
            .AllowAnonymous();

        admin.MapPut("/subscriptions", async (
            GrantSubscriptionCommand command,
            GrantSubscriptionCommandHandler handler,
            CancellationToken ct) =>
        {
            try
            {
                var result = await handler.HandleAsync(command, ct).ConfigureAwait(false);
                return Results.Ok(result);
            }
            catch (UserProfileNotFoundException)
            {
                return Results.NotFound();
            }
        });
    }
}
