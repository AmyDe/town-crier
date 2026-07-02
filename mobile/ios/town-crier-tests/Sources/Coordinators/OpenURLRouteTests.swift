import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// tc-28x2, GH #763 Problem 1 (second attempt). Root cause: in a
/// SwiftUI-lifecycle app, inbound Universal Links are delivered to
/// `.onOpenURL`, not `.onContinueUserActivity` -- but `.onOpenURL`
/// (TownCrierApp.swift) unconditionally handed every URL to
/// `AuthCallbackHandler`, silently dropping share links.
///
/// `OpenURLRoute.resolve` is the pure decision `.onOpenURL` now delegates
/// to: parse the URL as a Universal Link first, otherwise treat it as an
/// Auth0 callback. Extracted here (rather than left inline in the app
/// target's closure) because `swift test` cannot exercise
/// `town-crier-app/Sources` at all -- this is the only way to drive the
/// routing decision under TDD.
@Suite("OpenURLRoute")
struct OpenURLRouteTests {
  @Test func resolve_shareURL_returnsUniversalLinkWithShareApplicationDeepLink() throws {
    let url = try #require(
      URL(string: "https://share.towncrierapp.uk/a/kingston/Kingston/25/02755/CLC"))

    let result = OpenURLRoute.resolve(url)

    #expect(
      result
        == .universalLink(.shareApplication(authoritySlug: "kingston", ref: "Kingston/25/02755/CLC"))
    )
  }

  @Test func resolve_legacyApplicationURL_returnsUniversalLinkWithApplicationDetailDeepLink() throws {
    let url = try #require(URL(string: "https://towncrierapp.uk/applications/19/00123/FUL"))

    let result = OpenURLRoute.resolve(url)

    #expect(
      result == .universalLink(.applicationDetail(PlanningApplicationId(authority: "19", name: "00123/FUL"))))
  }

  @Test func resolve_auth0CallbackURL_returnsOther() throws {
    // Regression guard: an Auth0 login/logout callback must fall through to
    // AuthCallbackHandler, never be swallowed as a Universal Link.
    let url = try #require(
      URL(
        string:
          "uk.towncrierapp.mobile://towncrierapp.uk.auth0.com/ios/uk.towncrierapp.mobile/callback"
      ))

    let result = OpenURLRoute.resolve(url)

    #expect(result == .other(url))
  }

  @Test func resolve_unrecognisedURL_returnsOther() throws {
    let url = try #require(URL(string: "https://towncrierapp.uk/foo"))

    let result = OpenURLRoute.resolve(url)

    #expect(result == .other(url))
  }
}
