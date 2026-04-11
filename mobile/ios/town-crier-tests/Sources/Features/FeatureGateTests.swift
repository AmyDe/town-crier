import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("FeatureGate")
struct FeatureGateTests {

  // MARK: - Entitlement checks

  @Test func hasEntitlement_freeUser_searchApplications_returnsFalse() {
    let sut = FeatureGate(tier: .free)

    #expect(!sut.hasEntitlement(.searchApplications))
  }

  @Test func hasEntitlement_freeUser_statusChangeAlerts_returnsFalse() {
    let sut = FeatureGate(tier: .free)

    #expect(!sut.hasEntitlement(.statusChangeAlerts))
  }

  @Test func hasEntitlement_freeUser_decisionUpdateAlerts_returnsFalse() {
    let sut = FeatureGate(tier: .free)

    #expect(!sut.hasEntitlement(.decisionUpdateAlerts))
  }

  @Test func hasEntitlement_personalUser_statusChangeAlerts_returnsTrue() {
    let sut = FeatureGate(tier: .personal)

    #expect(sut.hasEntitlement(.statusChangeAlerts))
  }

  @Test func hasEntitlement_personalUser_searchApplications_returnsFalse() {
    let sut = FeatureGate(tier: .personal)

    #expect(!sut.hasEntitlement(.searchApplications))
  }

  @Test func hasEntitlement_proUser_searchApplications_returnsTrue() {
    let sut = FeatureGate(tier: .pro)

    #expect(sut.hasEntitlement(.searchApplications))
  }

  @Test func hasEntitlement_proUser_allEntitlements_returnsTrue() {
    let sut = FeatureGate(tier: .pro)

    for entitlement in Entitlement.allCases {
      #expect(sut.hasEntitlement(entitlement))
    }
  }

  // MARK: - Quota checks

  @Test func canAdd_freeUser_noZones_returnsTrue() {
    let sut = FeatureGate(tier: .free)

    #expect(sut.canAdd(quota: .watchZones, currentCount: 0))
  }

  @Test func canAdd_freeUser_oneZone_returnsFalse() {
    let sut = FeatureGate(tier: .free)

    #expect(!sut.canAdd(quota: .watchZones, currentCount: 1))
  }

  @Test func canAdd_personalUser_threeZones_returnsFalse() {
    let sut = FeatureGate(tier: .personal)

    #expect(!sut.canAdd(quota: .watchZones, currentCount: 3))
  }

  @Test func canAdd_proUser_manyZones_returnsTrue() {
    let sut = FeatureGate(tier: .pro)

    #expect(sut.canAdd(quota: .watchZones, currentCount: 100))
  }

  // MARK: - Upgrade badge

  @Test func shouldShowUpgradeBadge_entitlement_freeUserLacksSearch_returnsTrue() {
    let sut = FeatureGate(tier: .free)

    #expect(sut.shouldShowUpgradeBadge(for: .searchApplications))
  }

  @Test func shouldShowUpgradeBadge_entitlement_proUserHasSearch_returnsFalse() {
    let sut = FeatureGate(tier: .pro)

    #expect(!sut.shouldShowUpgradeBadge(for: .searchApplications))
  }

  @Test func shouldShowUpgradeBadge_quota_freeAtLimit_returnsTrue() {
    let sut = FeatureGate(tier: .free)

    #expect(sut.shouldShowUpgradeBadge(for: .watchZones, currentCount: 1))
  }

  @Test func shouldShowUpgradeBadge_quota_freeNotAtLimit_returnsFalse() {
    let sut = FeatureGate(tier: .free)

    #expect(!sut.shouldShowUpgradeBadge(for: .watchZones, currentCount: 0))
  }

  @Test func shouldShowUpgradeBadge_quota_proNeverShowsBadge_returnsFalse() {
    let sut = FeatureGate(tier: .pro)

    #expect(!sut.shouldShowUpgradeBadge(for: .watchZones, currentCount: 100))
  }

  // MARK: - Tier exposure

  @Test func tier_exposesInitializedTier() {
    let sut = FeatureGate(tier: .personal)

    #expect(sut.tier == .personal)
  }
}
