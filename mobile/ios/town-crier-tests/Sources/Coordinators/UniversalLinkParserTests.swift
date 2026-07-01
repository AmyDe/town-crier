import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("UniversalLinkParser")
struct UniversalLinkParserTests {
  @Test func parse_applicationDetailURL_returnsApplicationDetailDeepLink() throws {
    let url = try #require(URL(string: "https://towncrierapp.uk/applications/19/00123/FUL"))

    let result = UniversalLinkParser.parse(url)

    // URL path /applications/19/00123/FUL → authority "19", name "00123/FUL"
    #expect(result == .applicationDetail(PlanningApplicationId(authority: "19", name: "00123/FUL")))
  }

  @Test func parse_applicationsRootURL_returnsApplicationsListDeepLink() throws {
    let url = try #require(URL(string: "https://towncrierapp.uk/applications"))

    let result = UniversalLinkParser.parse(url)

    #expect(result == .applicationsList)
  }

  @Test func parse_applicationsRootWithTrailingSlash_returnsApplicationsListDeepLink() throws {
    let url = try #require(URL(string: "https://towncrierapp.uk/applications/"))

    let result = UniversalLinkParser.parse(url)

    #expect(result == .applicationsList)
  }

  @Test func parse_unrecognisedPath_returnsNil() throws {
    let url = try #require(URL(string: "https://towncrierapp.uk/foo"))

    let result = UniversalLinkParser.parse(url)

    #expect(result == nil)
  }

  @Test func parse_emptyPath_returnsNil() throws {
    let url = try #require(URL(string: "https://towncrierapp.uk"))

    let result = UniversalLinkParser.parse(url)

    #expect(result == nil)
  }

  @Test func parse_applicationsPrefixWithoutSeparator_returnsNil() throws {
    // Guard against false-positive matches like `/applicationsfoo`.
    let url = try #require(URL(string: "https://towncrierapp.uk/applicationsfoo"))

    let result = UniversalLinkParser.parse(url)

    #expect(result == nil)
  }

  // MARK: - Public share scheme /a/{authoritySlug}/{ref...} (GH #738 Slice 4)

  @Test func parse_shareURLWithPrefixedRef_returnsShareApplicationDeepLink() throws {
    // The ref is the application's full area-prefixed PlanIt name, verbatim —
    // it contains slashes, which are preserved as-is after the slug segment.
    let url = try #require(
      URL(string: "https://share.towncrierapp.uk/a/kingston/Kingston/25/02755/CLC"))

    let result = UniversalLinkParser.parse(url)

    #expect(result == .shareApplication(authoritySlug: "kingston", ref: "Kingston/25/02755/CLC"))
  }

  @Test func parse_shareURLWithSimpleRef_returnsShareApplicationDeepLink() throws {
    let url = try #require(URL(string: "https://share.towncrierapp.uk/a/croydon/23/03456/FUL"))

    let result = UniversalLinkParser.parse(url)

    #expect(result == .shareApplication(authoritySlug: "croydon", ref: "23/03456/FUL"))
  }

  @Test func parse_sharePrefixWithoutSeparator_returnsNil() throws {
    // Guard against false-positive matches like `/afoo`: the `/a/` separator is
    // required, so a path that merely starts with `/a` must not match.
    let url = try #require(URL(string: "https://share.towncrierapp.uk/afoo"))

    let result = UniversalLinkParser.parse(url)

    #expect(result == nil)
  }

  @Test func parse_shareBarePathNoRef_returnsNil() throws {
    // `/a` carries neither a slug nor a ref.
    let url = try #require(URL(string: "https://share.towncrierapp.uk/a"))

    let result = UniversalLinkParser.parse(url)

    #expect(result == nil)
  }

  @Test func parse_shareTrailingSlashNoRef_returnsNil() throws {
    // `/a/` has a separator but no slug/ref.
    let url = try #require(URL(string: "https://share.towncrierapp.uk/a/"))

    let result = UniversalLinkParser.parse(url)

    #expect(result == nil)
  }

  @Test func parse_shareSlugWithoutRef_returnsNil() throws {
    // A slug with no ref segment after it is not a valid share link.
    let url = try #require(URL(string: "https://share.towncrierapp.uk/a/kingston"))

    let result = UniversalLinkParser.parse(url)

    #expect(result == nil)
  }
}
