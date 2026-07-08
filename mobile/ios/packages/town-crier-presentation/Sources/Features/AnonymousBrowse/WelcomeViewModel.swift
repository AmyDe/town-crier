import Foundation

/// Backs the anonymous browse flow's welcome screen (GH#868 Phase 3): a thin
/// ViewModel whose only job is to expose the two entry intents. Navigation is
/// decided by ``AnonymousBrowseCoordinator``, which wires the callbacks —
/// mirrors ``MapViewModel/onApplicationSelected``.
@MainActor
public final class WelcomeViewModel: ObservableObject {
  private let onGetStarted: (() -> Void)?
  private let onSignIn: (() -> Void)?

  public init(onGetStarted: (() -> Void)? = nil, onSignIn: (() -> Void)? = nil) {
    self.onGetStarted = onGetStarted
    self.onSignIn = onSignIn
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
}
