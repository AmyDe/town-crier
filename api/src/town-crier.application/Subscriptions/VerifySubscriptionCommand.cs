namespace TownCrier.Application.Subscriptions;

public sealed record VerifySubscriptionCommand(string UserId, string SignedTransaction);
