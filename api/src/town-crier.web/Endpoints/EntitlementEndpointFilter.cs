using TownCrier.Domain.Entitlements;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Web.Endpoints;

internal sealed class EntitlementEndpointFilter : IEndpointFilter
{
    public async ValueTask<object?> InvokeAsync(
        EndpointFilterInvocationContext context,
        EndpointFilterDelegate next)
    {
        var endpoint = context.HttpContext.GetEndpoint();
        var attribute = endpoint?.Metadata.GetMetadata<RequiresEntitlementAttribute>();
        if (attribute is null)
        {
            return await next(context).ConfigureAwait(false);
        }

        var tierClaim = context.HttpContext.User.FindFirst("subscription_tier")?.Value;
        var tier = Enum.TryParse<SubscriptionTier>(tierClaim, ignoreCase: true, out var parsed)
            ? parsed
            : SubscriptionTier.Free;

        var entitlements = EntitlementMap.EntitlementsFor(tier);
        if (!entitlements.Contains(attribute.Entitlement))
        {
            return Results.Json(
                new EntitlementErrorResponse(
                    "insufficient_entitlement",
                    attribute.Entitlement.ToString(),
                    "This feature requires a paid subscription."),
                AppJsonSerializerContext.Default.EntitlementErrorResponse,
                statusCode: 403);
        }

        return await next(context).ConfigureAwait(false);
    }
}
