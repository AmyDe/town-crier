import Combine
import Foundation

/// Backs the anonymous browse flow's welcome screen (GH#868 Phase 3): a thin
/// ViewModel whose only job is to expose the two entry intents. Navigation is
/// decided by ``AnonymousBrowseCoordinator``, which wires the callbacks —
/// mirrors ``MapViewModel/onApplicationSelected``.
///
/// Also exposes the appearance control (GH#878): `appearanceMode` and
/// `selectAppearanceMode(_:)` forward directly to the injected
/// ``AppearanceStore`` — the same single live instance `SettingsViewModel`
/// reads and writes — so a mode picked here is immediately what the root
/// `.preferredColorScheme` and the Settings picker both see, with no second
/// copy of the state to diverge.
@MainActor
public final class WelcomeViewModel: ObservableObject {
  private let onGetStarted: (() -> Void)?
  private let onSignIn: (() -> Void)?
  private let appearanceStore: AppearanceStore
  // Forwards the shared store's own `objectWillChange` into this ViewModel's
  // — without this, `WelcomeView` (which observes `self`, not the store
  // directly) never re-renders when the mode changes from elsewhere (e.g.
  // Settings), so the welcome Menu's Picker shows a stale checkmark on
  // reopen even though `appearanceMode` itself always reads the live value.
  private var appearanceStoreSubscription: AnyCancellable?

  public init(
    appearanceStore: AppearanceStore? = nil,
    onGetStarted: (() -> Void)? = nil,
    onSignIn: (() -> Void)? = nil
  ) {
    let store = appearanceStore ?? AppearanceStore()
    self.appearanceStore = store
    self.onGetStarted = onGetStarted
    self.onSignIn = onSignIn
    appearanceStoreSubscription = store.objectWillChange.sink { [weak self] in
      self?.objectWillChange.send()
    }
  }

  /// "Get started" — routes to postcode entry. Deliberately never calls Auth0;
  /// anonymous browsing requires no account.
  public func getStarted() {
    onGetStarted?()
  }

  /// "I already have an account" — routes to the existing Auth0 login flow.
  public func signIn() {
    onSignIn?()
  }

  /// The currently active appearance mode — a live read-through to the
  /// shared ``AppearanceStore``, never a separate copy.
  public var appearanceMode: AppearanceMode {
    appearanceStore.appearanceMode
  }

  /// Picks a new appearance mode from the welcome screen's Menu. Writes
  /// straight through to the shared store, which persists it and — because
  /// the store is the same instance the app root observes — live-updates the
  /// colour scheme immediately.
  public func selectAppearanceMode(_ mode: AppearanceMode) {
    appearanceStore.appearanceMode = mode
  }
}
