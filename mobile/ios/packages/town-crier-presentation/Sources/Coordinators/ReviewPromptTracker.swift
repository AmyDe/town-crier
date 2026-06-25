import Foundation
import TownCrierDomain

/// Performs the real App Store review request. Abstracted behind a protocol so
/// the decision flow stays unit-testable (the OS call sits at the app-layer
/// edge — see ``CoordinatorReviewRequester``).
@MainActor
public protocol ReviewRequesting {
  func requestReview()
}

/// Production ``ReviewRequesting`` that flips ``AppCoordinator/isRequestingReview``.
///
/// An app-layer `.onChange(of:)` observes that flag, performs the real
/// `requestReview` via the SwiftUI environment, then resets it — mirroring the
/// established `isOpeningSystemNotificationSettings` → `openURL` pattern. The
/// coordinator reference is weak to avoid a retain cycle (the coordinator owns
/// the tracker, which owns this requester).
@MainActor
public final class CoordinatorReviewRequester: ReviewRequesting {
  public weak var coordinator: AppCoordinator?

  public init(coordinator: AppCoordinator? = nil) {
    self.coordinator = coordinator
  }

  public func requestReview() {
    coordinator?.isRequestingReview = true
  }
}

/// The @MainActor service the app talks to for the App Store review prompt
/// (GH #628).
///
/// It owns the only mutable session flag (``suppressThisSession()``), reads and
/// writes the persisted state via the injected ``ReviewPromptStore``, runs the
/// pure ``ReviewPromptPolicy`` on each signal, and — on a `.fire` decision —
/// asks the injected ``ReviewRequesting`` to present the dialog. Being
/// `@MainActor`, back-to-back signals are serialized, so each is evaluated once.
@MainActor
public final class ReviewPromptTracker {
  private let store: ReviewPromptStore
  private let policy: ReviewPromptPolicy
  private let requester: any ReviewRequesting
  private let now: @Sendable () -> Date
  private var isSessionSuppressed = false

  public init(
    store: ReviewPromptStore,
    requester: any ReviewRequesting,
    now: @escaping @Sendable () -> Date = Date.init,
    policy: ReviewPromptPolicy? = nil
  ) {
    self.store = store
    self.requester = requester
    self.now = now
    self.policy = policy ?? ReviewPromptPolicy(now: now)
    establishFirstLaunchDateIfNeeded()
  }

  /// Suppresses any review prompt for the rest of this session. Called during
  /// onboarding, when the post-purchase push prompt fires, and on friction.
  public func suppressThisSession() {
    isSessionSuppressed = true
  }

  /// Records an engagement signal: updates the persisted state via the policy
  /// and, on a `.fire`, requests the native review dialog.
  public func record(_ signal: ReviewSignal, isReactivation: Bool = false) {
    let outcome = policy.evaluate(
      signal: signal,
      state: store.load(),
      sessionSuppressed: isSessionSuppressed,
      isReactivation: isReactivation
    )
    store.save(outcome.state)
    if outcome.decision == .fire {
      requester.requestReview()
    }
  }

  /// Records a loyalty active-day signal on app foreground. `isReactivation` is
  /// `true` only for a background→active re-entry (never the cold-launch render).
  public func recordAppForegrounded(isReactivation: Bool) {
    record(.activeDay, isReactivation: isReactivation)
  }

  /// Anchors the account-age guard the first time the tracker runs on a device.
  private func establishFirstLaunchDateIfNeeded() {
    var state = store.load()
    guard state.firstLaunchDate == nil else { return }
    state.firstLaunchDate = now()
    store.save(state)
  }
}
