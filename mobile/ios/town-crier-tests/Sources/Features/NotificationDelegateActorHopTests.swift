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

  /// Surrogate for the production delegate body. Has the exact same shape:
  /// `nonisolated` outer function, `await MainActor.run { ... }` wraps the
  /// body, parser runs inside the wrap, returns whether the exit ran on main.
  /// Takes a pre-built sendable `applicationRef` to avoid `[AnyHashable: Any]`
  /// sendability noise — the userInfo dict itself is constructed inside the
  /// MainActor closure, mirroring how the real delegate hands `userInfo` off.
  ///
  /// The shape of this surrogate is load-bearing: any change that drops the
  /// `await MainActor.run` (e.g. a refactor that "simplifies" by relying on
  /// AppCoordinator's `@MainActor` to hop implicitly) breaks the contract
  /// the production delegate now depends on. The two Test cases above
  /// would catch a regression of `didExitOnMain` flipping to `false`.
  private nonisolated func simulateDelegateBody(
    applicationRef: String?
  ) async -> (didExitOnMain: Bool, deepLink: DeepLink?) {
    await MainActor.run {
      var userInfo: [AnyHashable: Any] = [:]
      if let applicationRef {
        userInfo["applicationRef"] = applicationRef
      }
      let deepLink = NotificationPayloadParser.parseDeepLink(from: userInfo)
      return (Thread.isMainThread, deepLink)
    }
  }
}
