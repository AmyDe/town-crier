import Foundation
import Testing
import TownCrierData

@Suite("APIEnvironment")
struct APIEnvironmentTests {

  @Test func development_baseURL_pointsToDevDomain() {
    let sut = APIEnvironment.development

    #expect(sut.baseURL.absoluteString == "https://api-dev.towncrierapp.uk")
  }

  @Test func production_baseURL_pointsToProductionDomain() {
    let sut = APIEnvironment.production

    #expect(sut.baseURL.absoluteString == "https://api.towncrierapp.uk")
  }

  @Test func current_returnsDevelopmentInDebug_productionInRelease() {
    let current = APIEnvironment.current

    #if DEBUG
      #expect(current == .development)
    #else
      #expect(current == .production)
    #endif
  }
}
