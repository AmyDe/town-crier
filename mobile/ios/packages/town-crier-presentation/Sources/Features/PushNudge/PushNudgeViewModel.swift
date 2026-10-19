import Foundation
import TownCrierDomain

/// ViewModel for the home-tab push-permission nudge banner. `.denied` can only
/// deep-link to iOS Settings because the system prompt cannot be re-shown once
/// denied.
@MainActor
public final class PushNudgeViewModel: ObservableObject {
  /// Reflects the system notification permission. `nil` until the first
  /// ``load()`` completes — the view treats `nil` as "hidden, no flash".
  @Published public private(set) var authorizationStatus: NotificationAuthorizationStatus?

  private let tier: SubscriptionTier
  private let notificationService: NotificationService
  private let onOpenSettings: () -> Void

  public init(
    tier: SubscriptionTier,
    notificationService: NotificationService,
    onOpenSettings: @escaping () -> Void
  ) {
    self.tier = tier
    self.notificationService = notificationService
    self.onOpenSettings = onOpenSettings
  }

  /// `true` only for a paid tier whose resolved authorization is not
  /// `.authorized`; hidden while the status is still `nil`.
  public var isVisible: Bool {
    guard tier > .free, let authorizationStatus else { return false }
    return authorizationStatus != .authorized
  }

  /// Banner body copy, status-appropriate.
  public var bodyText: String {
    switch authorizationStatus {
    case .denied:
      return "Notifications are switched off in iOS Settings. Tap to turn them back on."
    default:
      return "Notifications are off. Turn them on to get the instant alerts you're paying for."
    }
  }

  /// Primary-action button title, status-appropriate.
  public var buttonTitle: String {
    switch authorizationStatus {
    case .denied:
      return "Open Settings"
    default:
      return "Turn on"
    }
  }

  /// Reads the current authorization status on appear.
  public func load() async {
    authorizationStatus = await notificationService.authorizationStatus()
  }

  /// Re-reads the authorization status without prompting.
  public func refresh() async {
    authorizationStatus = await notificationService.authorizationStatus()
  }

  /// Acts on the current status: `.notDetermined` requests the system prompt
  /// and re-reads the resulting status; `.denied` deep-links to iOS Settings.
  public func primaryAction() async {
    switch authorizationStatus {
    case .notDetermined:
      _ = try? await notificationService.requestPermission()
      authorizationStatus = await notificationService.authorizationStatus()
    case .denied:
      onOpenSettings()
    case .authorized, .none:
      break
    }
  }
}
