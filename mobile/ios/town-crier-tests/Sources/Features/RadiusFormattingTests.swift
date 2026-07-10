import Foundation
import Testing

@testable import TownCrierPresentation

@Suite("formatRadius")
struct RadiusFormattingTests {

  @Test func formatRadius_under1000_showsMetres() {
    #expect(formatRadius(500) == "500m")
  }

  @Test func formatRadius_exactly1000_showsWholeKm() {
    #expect(formatRadius(1000) == "1km")
  }

  @Test func formatRadius_over1000_wholeKm_showsWholeKm() {
    #expect(formatRadius(2000) == "2km")
  }

  @Test func formatRadius_fractionalKm_showsOneDecimal() {
    #expect(formatRadius(1500) == "1.5km")
  }

  @Test func formatRadius_5000_shows5Km() {
    #expect(formatRadius(5000) == "5km")
  }

  @Test func formatRadius_smallValue_shows250m() {
    #expect(formatRadius(250) == "250m")
  }
}
