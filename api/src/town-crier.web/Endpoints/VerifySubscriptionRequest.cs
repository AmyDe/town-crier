namespace TownCrier.Web.Endpoints;

/// <summary>
/// Request body for <c>POST /v1/subscriptions/verify</c>.
/// </summary>
/// <remarks>
/// Two shapes are accepted:
/// <list type="bullet">
/// <item><description>A purchase supplies <see cref="SignedTransaction"/> — a
/// single Apple StoreKit 2 signed JWS transaction (compact serialization).</description></item>
/// <item><description>A restore supplies <see cref="SignedTransactions"/> — the
/// list of signed JWS transactions from <c>Transaction.currentEntitlements</c>,
/// which may include lapsed transactions.</description></item>
/// </list>
/// Exactly one of the two should be populated; if both are present they are
/// merged into a single verification set.
/// </remarks>
internal sealed record VerifySubscriptionRequest(
    string? SignedTransaction,
    IReadOnlyList<string>? SignedTransactions = null);
