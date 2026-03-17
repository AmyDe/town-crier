namespace TownCrier.Application.Subscriptions;

public enum NotificationOutcome
{
    Processed,
    InvalidSignature,
    UserNotFound,
}

public sealed record HandleAppStoreNotificationResult(NotificationOutcome Outcome);
