import Testing
import TownCrierDomain

@Suite("SubscriptionTier — featureHighlights")
struct SubscriptionTierFeatureHighlightsTests {
  @Test("free tier: 1 zone, 2 km radius, weekly email summary, no instant alerts")
  func free() {
    #expect(
      SubscriptionTier.free.featureHighlights == [
        "1 watch zone",
        "Zones up to 2 km radius",
        "Weekly email summary",
      ]
    )
  }

  @Test("personal tier: 3 zones, 5 km radius, alerts by push and email")
  func personal() {
    #expect(
      SubscriptionTier.personal.featureHighlights == [
        "3 watch zones",
        "Zones up to 5 km radius",
        "Status & decision alerts by push and email",
      ]
    )
  }

  @Test("pro tier: unlimited zones, 10 km radius, alerts by push and email")
  func pro() {
    #expect(
      SubscriptionTier.pro.featureHighlights == [
        "Unlimited watch zones",
        "Zones up to 10 km radius",
        "Status & decision alerts by push and email",
      ]
    )
  }
}
