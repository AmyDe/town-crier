using System.Security.Claims;
using TownCrier.Application.OfferCodes;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.OfferCodes;

namespace TownCrier.Web.Endpoints;

internal static class OfferCodeEndpoints
{
    public static void MapOfferCodeEndpoints(this RouteGroupBuilder group)
    {
        group.MapPost("/offer-codes/redeem", async (
            ClaimsPrincipal user,
            RedeemOfferCodeRequest request,
            RedeemOfferCodeCommandHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;

            try
            {
                var result = await handler
                    .HandleAsync(new RedeemOfferCodeCommand(userId, request.Code), ct)
                    .ConfigureAwait(false);

                return Results.Json(
                    new RedeemOfferCodeResponse(result.Tier.ToString(), result.ExpiresAt),
                    AppJsonSerializerContext.Default.RedeemOfferCodeResponse);
            }
            catch (InvalidOfferCodeFormatException ex)
            {
                return Results.Json(
                    new ApiErrorResponse("invalid_code_format", ex.Message),
                    AppJsonSerializerContext.Default.ApiErrorResponse,
                    statusCode: 400);
            }
            catch (OfferCodeNotFoundException ex)
            {
                return Results.Json(
                    new ApiErrorResponse("invalid_code", ex.Message),
                    AppJsonSerializerContext.Default.ApiErrorResponse,
                    statusCode: 404);
            }
            catch (OfferCodeAlreadyRedeemedException ex)
            {
                return Results.Json(
                    new ApiErrorResponse("code_already_redeemed", ex.Message),
                    AppJsonSerializerContext.Default.ApiErrorResponse,
                    statusCode: 409);
            }
            catch (AlreadySubscribedException ex)
            {
                return Results.Json(
                    new ApiErrorResponse("already_subscribed", ex.Message),
                    AppJsonSerializerContext.Default.ApiErrorResponse,
                    statusCode: 409);
            }
        });
    }
}
