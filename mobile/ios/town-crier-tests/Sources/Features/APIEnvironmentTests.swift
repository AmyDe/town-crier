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

  @Test func environment_sandboxReceipt_routesToDevelopment() {
    // TestFlight builds ship a sandbox App Store receipt.
    #expect(
      APIEnvironment.environment(forReceiptLastPathComponent: "sandboxReceipt") == .development)
  }

  @Test func environment_productionReceipt_routesToProduction() {
    // App Store builds ship a production receipt named "receipt".
    #expect(APIEnvironment.environment(forReceiptLastPathComponent: "receipt") == .production)
  }

  @Test func environment_missingReceipt_routesToProduction() {
    // Ad-hoc / local Release runs have no receipt; default to the safe prod choice.
    #expect(APIEnvironment.environment(forReceiptLastPathComponent: nil) == .production)
  }
}
