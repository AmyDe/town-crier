namespace TownCrier.Application.OfferCodes;

public sealed record RedeemOfferCodeCommand(string UserId, string Code);
