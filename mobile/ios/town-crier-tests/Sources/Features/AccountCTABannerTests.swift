import Testing

@testable import TownCrierPresentation

// Copy is a deliberate product/legal decision (GH#868 Phase 3.4) — these
// assertions guard against accidental drift, especially never re-introducing
// "instant" (a paid-tier word; free accounts get the weekly digest only).
@Suite("AccountCTABanner")
@MainActor
struct AccountCTABannerTests {
  @Test func copy_matchesExactApprovedText() {
    #expect(AccountCTABanner.Copy.headline == "Want to know when something changes here?")
    #expect(
      AccountCTABanner.Copy.subline
        == "Create a free account and we'll send you alerts for this area.")
    #expect(AccountCTABanner.Copy.createAccount == "Create free account")
    #expect(AccountCTABanner.Copy.signIn == "Sign in")
  }

  @Test func copy_neverMentionsInstant() {
    let allCopy = [
      AccountCTABanner.Copy.headline,
      AccountCTABanner.Copy.subline,
      AccountCTABanner.Copy.createAccount,
      AccountCTABanner.Copy.signIn,
    ].joined()

    #expect(!allCopy.lowercased().contains("instant"))
  }

  @Test func body_renders() {
    let sut = AccountCTABanner(onCreateAccount: {}, onSignIn: {})
    _ = sut.body
  }
}
