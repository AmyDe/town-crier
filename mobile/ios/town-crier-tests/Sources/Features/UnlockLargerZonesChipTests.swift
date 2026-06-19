import Testing

@testable import TownCrierPresentation

@MainActor
@Suite("UnlockLargerZonesChip")
struct UnlockLargerZonesChipTests {

  // The chip sells the whole upgrade, not just a bigger radius (tc-42gc). Copy
  // is exposed as constants so the value proposition lives in one place and can
  // be asserted directly.

  // MARK: - Title

  @Test func title_invitesAnUpgrade() {
    #expect(UnlockLargerZonesChip.Copy.title == "Do more with a plan")
  }

  // MARK: - Benefits line

  @Test func benefits_coverBiggerZones() {
    #expect(UnlockLargerZonesChip.Copy.benefits.contains("Bigger watch zones up to 10 km"))
  }

  @Test func benefits_coverMoreThanOneZone() {
    #expect(UnlockLargerZonesChip.Copy.benefits.contains("more than one zone"))
  }

  @Test func benefits_phrasePushAndEmailAsOneAlert() {
    // Push + email are two channels for the SAME alert, never two features.
    #expect(UnlockLargerZonesChip.Copy.benefits.contains("instant alerts by push and email"))
  }

  @Test func benefits_clarifyTheFreeTier() {
    #expect(UnlockLargerZonesChip.Copy.benefits.contains("Free gives you a weekly digest."))
  }

  @Test func benefits_exactWording() {
    #expect(
      UnlockLargerZonesChip.Copy.benefits
        == "Bigger watch zones up to 10 km, more than one zone, and instant alerts by "
          + "push and email. Free gives you a weekly digest."
    )
  }

  // MARK: - Accessibility

  @Test func accessibilityLabel_describesTheWholeUpgrade() {
    #expect(
      UnlockLargerZonesChip.Copy.accessibilityLabel
        == "Do more with a plan. Bigger watch zones up to 10 kilometres, more than one "
          + "watch zone, and instant alerts by push and email. Free gives you a weekly digest."
    )
  }

  @Test func accessibilityLabel_spellsOutKilometres() {
    #expect(UnlockLargerZonesChip.Copy.accessibilityLabel.contains("10 kilometres"))
  }

  @Test func accessibilityHint_opensPlans() {
    #expect(UnlockLargerZonesChip.Copy.accessibilityHint == "Opens subscription plans")
  }

  // MARK: - View

  @Test func body_renders() {
    let sut = UnlockLargerZonesChip(action: {})
    _ = sut.body
  }
}
