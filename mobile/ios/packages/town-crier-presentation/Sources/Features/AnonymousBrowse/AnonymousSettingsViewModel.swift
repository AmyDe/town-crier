import Combine
import Foundation
import TownCrierDomain

/// The anonymous (pre-signup) Settings tab's ViewModel (GH#879 Phase 3): a
/// deliberately small, distinct type from the authenticated
/// `SettingsViewModel` rather than a shared abstraction over the two — it has
/// no `AuthenticationService`, `SubscriptionService`, or
/// `UserProfileRepository` dependency at all, so account info, notification
/// preferences, subscription state, data export, sign-out, and account
/// deletion structurally cannot appear on this screen: there is no data
/// behind them.
///
/// Exposes exactly what an anonymous user's Settings needs: the shared
/// appearance preference (GH#878), the app version, and the same data
/// attribution rows the authenticated screen shows.
@MainActor
public final class AnonymousSettingsViewModel: ObservableObject {
  private let appearanceStore: AppearanceStore
  private let appVersionProvider: AppVersionProvider
  // Forwards the shared store's own `objectWillChange` into this ViewModel's
  // — without this, `AnonymousSettingsView` (which observes `self`, not the
  // store directly) never re-renders when the mode changes from elsewhere
  // (e.g. the welcome screen) — mirrors `SettingsViewModel`/`WelcomeViewModel`.
  private var appearanceStoreSubscription: AnyCancellable?

  public init(appearanceStore: AppearanceStore, appVersionProvider: AppVersionProvider) {
    self.appearanceStore = appearanceStore
    self.appVersionProvider = appVersionProvider
    appearanceStoreSubscription = appearanceStore.objectWillChange.sink { [weak self] in
      self?.objectWillChange.send()
    }
  }

  /// The currently active appearance mode — a live read-through to the
  /// shared ``AppearanceStore`` (GH#878), never a separate copy. Bound by the
  /// Settings picker via `$viewModel.appearanceMode`.
  public var appearanceMode: AppearanceMode {
    get { appearanceStore.appearanceMode }
    set { appearanceStore.appearanceMode = newValue }
  }

  public var appVersion: String {
    "\(appVersionProvider.version) (\(appVersionProvider.buildNumber))"
  }

  /// Same content as the authenticated `SettingsViewModel.attributionItems`
  /// — both read from the single shared ``AttributionItem/standard`` set so
  /// the two surfaces cannot drift apart.
  public var attributionItems: [AttributionItem] {
    AttributionItem.standard
  }
}
