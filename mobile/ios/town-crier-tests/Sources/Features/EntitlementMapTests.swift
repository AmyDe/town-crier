import Testing
import TownCrierDomain

// MARK: - entitlements(for:)

@Suite("EntitlementMap — entitlements(for:)")
struct EntitlementMapEntitlementsForTierTests {
    @Test("free tier has no entitlements")
    func freeTierEmpty() {
        let entitlements = EntitlementMap.entitlements(for: .free)
        #expect(entitlements.isEmpty)
    }

    @Test("personal tier includes statusChangeAlerts")
    func personalHasStatusChangeAlerts() {
        let entitlements = EntitlementMap.entitlements(for: .personal)
        #expect(entitlements.contains(.statusChangeAlerts))
    }

    @Test("personal tier includes decisionUpdateAlerts")
    func personalHasDecisionUpdateAlerts() {
        let entitlements = EntitlementMap.entitlements(for: .personal)
        #expect(entitlements.contains(.decisionUpdateAlerts))
    }

    @Test("personal tier includes hourlyDigestEmails")
    func personalHasHourlyDigestEmails() {
        let entitlements = EntitlementMap.entitlements(for: .personal)
        #expect(entitlements.contains(.hourlyDigestEmails))
    }

    @Test("personal tier does not include searchApplications")
    func personalDoesNotHaveSearch() {
        let entitlements = EntitlementMap.entitlements(for: .personal)
        #expect(!entitlements.contains(.searchApplications))
    }

    @Test("personal tier has exactly 3 entitlements")
    func personalHasThreeEntitlements() {
        let entitlements = EntitlementMap.entitlements(for: .personal)
        #expect(entitlements.count == 3)
    }

    @Test("pro tier includes all entitlements")
    func proHasAllEntitlements() {
        let entitlements = EntitlementMap.entitlements(for: .pro)
        for entitlement in Entitlement.allCases {
            #expect(entitlements.contains(entitlement))
        }
    }

    @Test("pro tier has exactly 4 entitlements")
    func proHasFourEntitlements() {
        let entitlements = EntitlementMap.entitlements(for: .pro)
        #expect(entitlements.count == 4)
    }
}

// MARK: - limit(for:quota:)

@Suite("EntitlementMap — limit(for:quota:)")
struct EntitlementMapLimitForTierQuotaTests {
    @Test("free tier gets 1 watch zone")
    func freeGetsOneWatchZone() {
        let limit = EntitlementMap.limit(for: .free, quota: .watchZones)
        #expect(limit == 1)
    }

    @Test("personal tier gets 3 watch zones")
    func personalGetsThreeWatchZones() {
        let limit = EntitlementMap.limit(for: .personal, quota: .watchZones)
        #expect(limit == 3)
    }

    @Test("pro tier gets unlimited watch zones")
    func proGetsUnlimitedWatchZones() {
        let limit = EntitlementMap.limit(for: .pro, quota: .watchZones)
        #expect(limit == Int.max)
    }
}

// MARK: - hasEntitlement(_:for:)

@Suite("EntitlementMap — hasEntitlement(_:for:)")
struct EntitlementMapHasEntitlementTests {
    @Test("free tier does not have searchApplications")
    func freeDoesNotHaveSearch() {
        #expect(!EntitlementMap.hasEntitlement(.searchApplications, for: .free))
    }

    @Test("personal tier has statusChangeAlerts")
    func personalHasStatusAlerts() {
        #expect(EntitlementMap.hasEntitlement(.statusChangeAlerts, for: .personal))
    }

    @Test("pro tier has searchApplications")
    func proHasSearch() {
        #expect(EntitlementMap.hasEntitlement(.searchApplications, for: .pro))
    }
}

// MARK: - canAdd(for:currentCount:quota:)

@Suite("EntitlementMap — canAdd(for:currentCount:quota:)")
struct EntitlementMapCanAddTests {
    @Test("free tier can add first watch zone")
    func freeCanAddFirst() {
        #expect(EntitlementMap.canAdd(for: .free, currentCount: 0, quota: .watchZones))
    }

    @Test("free tier cannot add second watch zone")
    func freeCannotAddSecond() {
        #expect(!EntitlementMap.canAdd(for: .free, currentCount: 1, quota: .watchZones))
    }

    @Test("personal tier can add up to 3 watch zones")
    func personalCanAddUpToThree() {
        #expect(EntitlementMap.canAdd(for: .personal, currentCount: 2, quota: .watchZones))
        #expect(!EntitlementMap.canAdd(for: .personal, currentCount: 3, quota: .watchZones))
    }

    @Test("pro tier can always add watch zones")
    func proCanAlwaysAdd() {
        #expect(EntitlementMap.canAdd(for: .pro, currentCount: 1000, quota: .watchZones))
    }
}
