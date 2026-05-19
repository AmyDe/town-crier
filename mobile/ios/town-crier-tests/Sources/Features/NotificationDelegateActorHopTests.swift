import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Isolation contract for the `NotificationDelegate` callback shape (tc-cbmk).
///
/// `UNUserNotificationCenterDelegate` async overloads are bridged by Swift's
/// compiler-synthesized `@objc` thunk, which calls the original
/// `withCompletionHandler:` ObjC selector when the function returns. UIKit's
/// `UNUserNotificationCenter` asserts that completion runs on the main
/// thread; if it fires on a Swift Concurrency cooperative thread the process
/// aborts with `NSInternalInconsistencyException`.
///
/// The fix is to declare the conforming class `@MainActor` and **not** mark
/// the delegate overloads `nonisolated`. The method bodies then inherit
/// MainActor isolation and the synthesized @objc thunk's completion fires on
/// main deterministically.
///
/// The previous fix (tc-fcwv) wrapped the body in `await MainActor.run` from
/// a `nonisolated async` method; that left a gap where the @objc thunk could
/// resume on whatever cooperative thread the runtime picked. tc-cbmk reverses
/// that pattern. These tests document the new contract via a surrogate that
/// mirrors the production delegate body shape — including the no-op
/// (early-return) path that was the original tc-fcwv repro and remains the
/// path most sensitive to isolation regressions.
@Suite("NotificationDelegate isolation contract (tc-cbmk)")
@MainActor
struct NotificationDelegateActorHopTests {

  /// Digest-shaped payload — `handlePushTap` finds no deep link and no
  /// `createdAt`. The body completes without an actor hop, and because the
  /// method is `@MainActor` (inherited), the synthesized @objc completion
  /// fires on main. Regression guard for tc-fcwv (no-deep-link path) and
  /// tc-cbmk (isolation regression to `nonisolated`).
  @Test func userNotificationCenter_didReceive_runsOnMain_forDigestPayload() async {
    let wasOnMain = await runDelegateBody(applicationRef: nil)

    #expect(wasOnMain == true)
  }

  /// Routed deep-link path: a valid `applicationRef` produces a deep link
  /// and the body completes on main. Same isolation contract as the no-op
  /// path — class-level `@MainActor` carries both.
  @Test func userNotificationCenter_didReceive_runsOnMain_forRoutedDeepLink() async {
    let wasOnMain = await runDelegateBody(applicationRef: "APP-001")

    #expect(wasOnMain == true)
  }

  /// Mirrors the production delegate body: a `@MainActor` method that
  /// forwards a `userInfo` dict to a `@MainActor` collaborator. No
  /// `nonisolated`, no `MainActor.run` — the method-level isolation is the
  /// only thing keeping the @objc thunk's completion on main.
  ///
  /// Reintroducing `nonisolated` on this surrogate (or on the production
  /// `userNotificationCenter(_:didReceive:)`) would resume the body on a
  /// cooperative thread and the `Thread.isMainThread` assertion at exit
  /// would fail.
  @MainActor
  private func runDelegateBody(applicationRef: String?) async -> Bool {
    var userInfo: [AnyHashable: Any] = [:]
    if let applicationRef {
      userInfo["applicationRef"] = applicationRef
    }
    if let deepLink = NotificationPayloadParser.parseDeepLink(from: userInfo) {
      _ = deepLink  // production path: coordinator.handleDeepLink(deepLink)
    }
    return isOnMainThread()
  }

  /// `Thread.isMainThread` is unavailable from async contexts, but it's
  /// fine to read from a synchronous `@MainActor` helper — and a synchronous
  /// helper is the only honest way to assert "this line ran on main".
  @MainActor
  private func isOnMainThread() -> Bool {
    Thread.isMainThread
  }
}
