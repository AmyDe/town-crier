import Testing

@testable import TownCrierPresentation

@MainActor
@Suite("CustomShapeAvailableCard")
struct CustomShapeAvailableCardTests {

  // Shown on the onboarding radius step in place of the old plain-text
  // "Draw a custom shape instead" button once the user's tier already allows
  // a custom-shape zone (tc-7se1w.4) — gives the capability the same
  // introduction weight ``CustomShapeUpsellCard`` gives a Free-tier user
  // pre-purchase, rather than a footnote link with no explanation.

  // MARK: - Copy

  @Test func title_isActionLed() {
    #expect(CustomShapeAvailableCard.Copy.title == "Draw a custom shape")
  }

  @Test func benefits_explainsTheAlternativeToACircle() {
    #expect(
      CustomShapeAvailableCard.Copy.benefits
        == "Trace the exact area you want to watch, instead of a circle."
    )
  }

  @Test func accessibilityLabel_combinesTitleAndBenefits() {
    #expect(
      CustomShapeAvailableCard.Copy.accessibilityLabel
        == "Draw a custom shape. Trace the exact area you want to watch, instead of a circle."
    )
  }

  @Test func accessibilityHint_opensTheDrawingTool() {
    #expect(CustomShapeAvailableCard.Copy.accessibilityHint == "Opens the shape drawing tool")
  }

  // MARK: - View

  @Test func body_renders() {
    let sut = CustomShapeAvailableCard {}
    _ = sut.body
  }
}
