namespace TownCrier.Web.Endpoints;

internal sealed record RedeemOfferCodeResponse(string Tier, DateTimeOffset ExpiresAt);
