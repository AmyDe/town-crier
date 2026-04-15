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

        admin.MapGet("/users", async (
            string? search,
            int? pageSize,
            string? continuationToken,
            ListUsersQueryHandler handler,
            CancellationToken ct) =>
        {
            var query = new ListUsersQuery(search, pageSize ?? 20, continuationToken);
            var result = await handler.HandleAsync(query, ct).ConfigureAwait(false);
            return Results.Ok(result);
        });
    }
}
