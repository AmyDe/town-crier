using System.Security.Claims;
using System.Text.Json;
using TownCrier.Application.Subscriptions;
using TownCrier.Application.UserProfiles;

namespace TownCrier.Web.Endpoints;

internal static class SubscriptionEndpoints
{
    public static void MapSubscriptionEndpoints(this RouteGroupBuilder group)
    {
        // The request body is read and deserialized explicitly (rather than via
        // minimal-API parameter binding) so a malformed JSON body returns a
        // clean 400 instead of bubbling a BadHttpRequestException up to the
        // global error handler as a 500.
        group.MapPost("/subscriptions/verify", async (
            HttpContext context,
            ClaimsPrincipal user,
            VerifySubscriptionCommandHandler handler,
            CancellationToken ct) =>
        {
            VerifySubscriptionRequest? request;
            try
            {
                request = await context.Request
                    .ReadFromJsonAsync(
                        AppJsonSerializerContext.Default.VerifySubscriptionRequest, ct)
                    .ConfigureAwait(false);
            }
            catch (JsonException)
            {
                return MalformedBody();
            }

            if (request is null || string.IsNullOrWhiteSpace(request.SignedTransaction))
            {
                return MalformedBody();
            }

            var userId = user.FindFirstValue("sub")!;

            try
            {
                var result = await handler
                    .HandleAsync(new VerifySubscriptionCommand(userId, request.SignedTransaction), ct)
                    .ConfigureAwait(false);

                return Results.Json(
                    new VerifySubscriptionResponse(
                        result.Tier.ToString(),
                        result.SubscriptionExpiry,
                        result.Entitlements,
                        result.WatchZoneLimit),
                    AppJsonSerializerContext.Default.VerifySubscriptionResponse);
            }
            catch (AppleJwsVerificationException ex)
            {
                return Results.Json(
                    new ApiErrorResponse("invalid_transaction", ex.Message),
                    AppJsonSerializerContext.Default.ApiErrorResponse,
                    statusCode: 401);
            }
            catch (ArgumentException ex)
            {
                return Results.Json(
                    new ApiErrorResponse("invalid_transaction_payload", ex.Message),
                    AppJsonSerializerContext.Default.ApiErrorResponse,
                    statusCode: 400);
            }
            catch (UserProfileNotFoundException ex)
            {
                return Results.Json(
                    new ApiErrorResponse("user_not_found", ex.Message),
                    AppJsonSerializerContext.Default.ApiErrorResponse,
                    statusCode: 404);
            }
        });
    }

    private static IResult MalformedBody() => Results.Json(
        new ApiErrorResponse("malformed_request", "The request body is not valid JSON."),
        AppJsonSerializerContext.Default.ApiErrorResponse,
        statusCode: 400);
}
