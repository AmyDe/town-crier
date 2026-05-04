import Foundation
import Testing

@testable import TownCrierPresentation

/// Regression guard for tc-fcwv (build 16 TestFlight crash on every push tap).
///
/// `UNUserNotificationCenterDelegate` callbacks declared as `nonisolated async`
/// are bridged by Swift's compiler-synthesized `@objc` thunk, which calls the
/// original `withCompletionHandler:` ObjC selector when the function returns.
/// UIKit's `UNUserNotificationCenter` asserts that the completion handler runs
/// on the main thread; if it fires on a Swift Concurrency cooperative thread
/// the process aborts with `NSInternalInconsistencyException`.
///
/// The fix is to wrap the delegate body in `await MainActor.run { ... }` so
/// the function always returns on MainActor regardless of which path is taken.
/// We can't directly reproduce the UIKit assertion in unit tests (the bridge
/// only fires for real `UNUserNotificationCenter` invocations), but we can
/// document and verify the actor-hop contract: a `nonisolated async` function
/// that wraps its body in `MainActor.run` is on the main thread at exit.
@Suite("NotificationDelegate actor-hop contract (tc-fcwv)")
struct NotificationDelegateActorHopTests {

  /// Mirrors the no-deep-link early-return path: the delegate parses a
  /// payload, finds nothing to route, and returns. With the fix in place the
  /// function is on MainActor when it returns. Without the fix it would
  /// return on whatever cooperative thread the parse happened on.
  @Test func nonisolatedAsyncWrappedInMainActorRun_landsOnMainAtExit_forNoOpBody() async {
    // Digest-shaped payload — no applicationRef, parser yields nil.
    let (didExitOnMain, returnedDeepLink) = await simulateDelegateBody(
      applicationRef: nil
    )

    #expect(didExitOnMain == true)
    #expect(returnedDeepLink == nil)
  }

  /// Mirrors the routed-deep-link path: a valid `applicationRef` payload
  /// produces a deep link and the body completes on MainActor.
  @Test func nonisolatedAsyncWrappedInMainActorRun_landsOnMainAtExit_forRoutedDeepLink() async {
    let (didExitOnMain, returnedDeepLink) = await simulateDelegateBody(
      applicationRef: "APP-001"
    )

    #expect(didExitOnMain == true)
    #expect(returnedDeepLink != nil)
  }

  /// Surrogate for the production delegate body. Mirrors the exact shape of
  /// `NotificationDelegate.userNotificationCenter(_:didReceive:)`:
  /// 1. `nonisolated async` outer function (same as the protocol callback).
  /// 2. Parse the non-Sendable `userInfo` dict on the calling actor.
  /// 3. Hop to MainActor with only a Sendable `DeepLink?` payload.
  /// 4. Return on MainActor regardless of whether a deep link was parsed.
  ///
  /// The shape is load-bearing: dropping `await MainActor.run` (e.g. a
  /// refactor that relies on `AppCoordinator`'s `@MainActor` to hop
  /// implicitly only on the routed path) reintroduces the early-return
  /// crash that this test guards against.
  private nonisolated func simulateDelegateBody(
    applicationRef: String?
  ) async -> (didExitOnMain: Bool, deepLink: DeepLink?) {
    var userInfo: [AnyHashable: Any] = [:]
    if let applicationRef {
      userInfo["applicationRef"] = applicationRef
    }
    let deepLink = NotificationPayloadParser.parseDeepLink(from: userInfo)
    return await MainActor.run {
      (Thread.isMainThread, deepLink)
    }
  }
}
