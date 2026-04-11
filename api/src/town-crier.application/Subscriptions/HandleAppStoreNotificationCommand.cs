namespace TownCrier.Application.Subscriptions;

public sealed record HandleAppStoreNotificationCommand(string SignedPayload);
