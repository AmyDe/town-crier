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
}
