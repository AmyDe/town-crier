using TownCrier.Application.Geocoding;

namespace TownCrier.Web.Endpoints;

internal static class GeocodeEndpoints
{
    public static void MapGeocodeEndpoints(this RouteGroupBuilder group)
    {
        group.MapGet("/geocode/{postcode}", async (
            string postcode,
            GeocodePostcodeQueryHandler handler,
            CancellationToken ct) =>
        {
            try
            {
                var result = await handler.HandleAsync(new GeocodePostcodeQuery(postcode), ct).ConfigureAwait(false);
                return Results.Ok(result);
            }
            catch (ArgumentException ex)
            {
                return Results.BadRequest(new ApiErrorResponse(ex.Message));
            }
            catch (InvalidOperationException ex)
            {
                return Results.NotFound(new ApiErrorResponse(ex.Message));
            }
        });
    }
}
