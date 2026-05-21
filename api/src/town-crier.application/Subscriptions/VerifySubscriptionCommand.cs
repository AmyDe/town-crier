namespace TownCrier.Application.Subscriptions;

/// <summary>
/// Verifies one or more Apple StoreKit 2 signed JWS transactions for a user and
/// applies the resulting entitlement to their Cosmos profile.
/// </summary>
/// <remarks>
/// A single-element list is a purchase (one freshly bought transaction). A
/// multi-element list is a restore — <c>Transaction.currentEntitlements</c>
/// from the device, which may include lapsed transactions. The handler verifies
/// every JWS, ignores expired transactions, and resolves the user to the
/// highest still-active tier (or Free if none are active).
/// </remarks>
public sealed record VerifySubscriptionCommand
{
    public VerifySubscriptionCommand(string userId, string signedTransaction)
        : this(userId, new[] { signedTransaction })
    {
    }

    public VerifySubscriptionCommand(string userId, IReadOnlyList<string> signedTransactions)
    {
        this.UserId = userId;
        this.SignedTransactions = signedTransactions;
    }

    public string UserId { get; }

    public IReadOnlyList<string> SignedTransactions { get; }
}
