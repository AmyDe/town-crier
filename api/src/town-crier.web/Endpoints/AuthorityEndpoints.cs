using TownCrier.Application.Authorities;

namespace TownCrier.Web.Endpoints;

internal static class AuthorityEndpoints
{
    public static RouteGroupBuilder MapAuthorityEndpoints(this RouteGroupBuilder group)
    {
        group.MapGet("/authorities", async (
            string? search,
            GetAuthoritiesQueryHandler handler,
            CancellationToken ct) =>
        {
            var result = await handler.HandleAsync(new GetAuthoritiesQuery(search), ct).ConfigureAwait(false);
            return Results.Ok(result);
        }).AllowAnonymous();

        group.MapGet("/authorities/{id:int}", async (
            int id,
            GetAuthorityByIdQueryHandler handler,
            CancellationToken ct) =>
        {
            var result = await handler.HandleAsync(new GetAuthorityByIdQuery(id), ct).ConfigureAwait(false);
            return result is null ? Results.NotFound() : Results.Ok(result);
        }).AllowAnonymous();

        return group;
    }
}
