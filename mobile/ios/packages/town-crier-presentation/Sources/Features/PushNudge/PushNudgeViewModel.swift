import Foundation
import TownCrierDomain

/// ViewModel for the home-tab push-permission nudge banner (issue #624).
///
/// A paid user who never granted notification permission is paying for instant
/// alerts they will never receive. This banner is the persistent safety net: it
/// shows on the Applications tab whenever the user is on a paid tier and
/// notifications are not authorized, and its primary action branches on the
/// system authorization status (mirroring `NotificationPreferencesViewModel`):
///
/// - `.notDetermined` → request the system prompt, then re-read the status.
/// - `.denied` → invoke `onOpenSettings` (the system prompt can never be
///   re-shown once denied; the only recovery is the iOS Settings deep link).
///
/// `authorizationStatus` is `nil` until the first ``load()`` completes; the
/// banner stays hidden in that window to avoid a flash before the status
/// resolves.
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

  /// `true` only for a paid tier whose resolved authorization is not yet
  /// `.authorized`. Hidden while the status is still loading (`nil`) and for
  /// any free-tier user.
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

  /// Re-reads the authorization status without prompting. Called from the view
  /// on `scenePhase == .active` so the banner disappears once the user enables
  /// notifications in iOS Settings and returns.
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
