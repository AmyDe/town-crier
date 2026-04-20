namespace TownCrier.Application.UserProfiles;

public sealed record ExportUserDataResult(
    string UserId,
    string? Email,
    ExportedNotificationPreferences NotificationPreferences,
    ExportedSubscription Subscription,
    IReadOnlyList<ExportedWatchZone> WatchZones,
    IReadOnlyList<ExportedNotification> Notifications,
    IReadOnlyList<ExportedDecisionAlert> DecisionAlerts,
    IReadOnlyList<ExportedSavedApplication> SavedApplications,
    IReadOnlyList<ExportedDeviceRegistration> DeviceRegistrations,
    IReadOnlyList<ExportedOfferCodeRedemption> OfferCodeRedemptions);
