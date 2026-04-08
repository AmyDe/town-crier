using TownCrier.Application.Legal;

namespace TownCrier.Web.Endpoints;

internal static class LegalEndpoints
{
    public static void MapLegalEndpoints(this RouteGroupBuilder group)
    {
        group.MapGet("/legal/{documentType}", async (string documentType) =>
        {
            var result = await GetLegalDocumentQueryHandler.HandleAsync(
                new GetLegalDocumentQuery(documentType), CancellationToken.None).ConfigureAwait(false);

            return result is not null ? Results.Ok(result) : Results.NotFound();
        })
        .AllowAnonymous();
    }
}
