import Testing
@testable import TownCrierDomain

@Suite("WatchZoneLimits")
struct WatchZoneLimitsTests {
    @Test func freeTier_maxZonesIsOne() {
        let limits = WatchZoneLimits(tier: .free)
        #expect(limits.maxZones == 1)
    }

    @Test func personalTier_maxZonesIsThree() {
        let limits = WatchZoneLimits(tier: .personal)
        #expect(limits.maxZones == 3)
    }

    @Test func proTier_maxZonesIsUnlimited() {
        let limits = WatchZoneLimits(tier: .pro)
        #expect(limits.maxZones == .max)
    }

    @Test func freeTier_maxRadiusMetresIs2000() {
        let limits = WatchZoneLimits(tier: .free)
        #expect(limits.maxRadiusMetres == 2000)
    }

    @Test func personalTier_maxRadiusMetresIs5000() {
        let limits = WatchZoneLimits(tier: .personal)
        #expect(limits.maxRadiusMetres == 5000)
    }

    @Test func proTier_maxRadiusMetresIs10000() {
        let limits = WatchZoneLimits(tier: .pro)
        #expect(limits.maxRadiusMetres == 10000)
    }

    @Test func canAddZone_underLimit_returnsTrue() {
        let limits = WatchZoneLimits(tier: .personal)
        #expect(limits.canAddZone(currentCount: 0))
    }

    @Test func canAddZone_atLimit_returnsFalse() {
        let limits = WatchZoneLimits(tier: .personal)
        #expect(!limits.canAddZone(currentCount: 3))
    }

    @Test func canAddZone_proWithMany_returnsTrue() {
        let limits = WatchZoneLimits(tier: .pro)
        #expect(limits.canAddZone(currentCount: 50))
    }

    @Test func clampRadius_withinLimit_returnsOriginal() {
        let limits = WatchZoneLimits(tier: .personal)
        #expect(limits.clampRadius(3000) == 3000)
    }

    @Test func clampRadius_exceedsLimit_returnsMax() {
        let limits = WatchZoneLimits(tier: .personal)
        #expect(limits.clampRadius(8000) == 5000)
    }

    @Test func availableRadiusOptions_freeTier() {
        let limits = WatchZoneLimits(tier: .free)
        let options = limits.availableRadiusOptions
        #expect(options.allSatisfy { $0 <= 2000 })
        #expect(!options.isEmpty)
    }

    @Test func availableRadiusOptions_proTier_includesLargeRadii() {
        let limits = WatchZoneLimits(tier: .pro)
        let options = limits.availableRadiusOptions
        #expect(options.contains(10000))
    }
}
