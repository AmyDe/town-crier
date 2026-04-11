import Foundation
import Testing

@testable import TownCrierPresentation

@Suite("formatRadius")
struct RadiusFormattingTests {

  @Test func formatRadius_under1000_showsMetres() {
    #expect(formatRadius(500) == "500 m")
  }

  @Test func formatRadius_exactly1000_showsWholeKm() {
    #expect(formatRadius(1000) == "1 km")
  }

  @Test func formatRadius_over1000_wholeKm_showsWholeKm() {
    #expect(formatRadius(2000) == "2 km")
  }

  @Test func formatRadius_fractionalKm_showsOneDecimal() {
    #expect(formatRadius(1500) == "1.5 km")
  }

  @Test func formatRadius_5000_shows5Km() {
    #expect(formatRadius(5000) == "5 km")
  }

  @Test func formatRadius_smallValue_shows250m() {
    #expect(formatRadius(250) == "250 m")
  }
}
