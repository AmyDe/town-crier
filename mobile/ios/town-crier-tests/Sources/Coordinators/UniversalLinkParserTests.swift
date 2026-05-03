import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("UniversalLinkParser")
struct UniversalLinkParserTests {
  @Test func parse_applicationDetailURL_returnsApplicationDetailDeepLink() throws {
    let url = try #require(URL(string: "https://towncrierapp.uk/applications/19/00123/FUL"))

    let result = UniversalLinkParser.parse(url)

    #expect(result == .applicationDetail(PlanningApplicationId("19/00123/FUL")))
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
}
