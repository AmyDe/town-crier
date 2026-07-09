import Testing

@testable import TownCrierPresentation

/// Public Notice radius scale (GH#857): 3/6/12, sharper than the previous
/// 8/12/16 rounded-shape language.
@Suite("TCCornerRadius")
struct TCCornerRadiusTests {

  @Test func small_is3pt() {
    #expect(TCCornerRadius.small == 3)
  }

  @Test func medium_is6pt() {
    #expect(TCCornerRadius.medium == 6)
  }

  @Test func large_is12pt() {
    #expect(TCCornerRadius.large == 12)
  }
}
