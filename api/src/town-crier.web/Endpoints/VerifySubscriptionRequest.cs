namespace TownCrier.Web.Endpoints;

/// <summary>
/// Request body for <c>POST /v1/subscriptions/verify</c>. <see cref="SignedTransaction"/>
/// is the Apple StoreKit 2 signed JWS transaction (compact serialization).
/// </summary>
internal sealed record VerifySubscriptionRequest(string SignedTransaction);
