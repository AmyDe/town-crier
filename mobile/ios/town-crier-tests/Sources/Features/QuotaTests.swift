import Testing
import TownCrierDomain

@Suite("Quota")
struct QuotaTests {
    @Test("watchZones case exists")
    func watchZonesCaseExists() {
        let quota = Quota.watchZones
        #expect(quota == .watchZones)
    }

    @Test("conforms to Sendable")
    func sendableConformance() {
        let quota: any Sendable = Quota.watchZones
        #expect(quota is Quota)
    }

    @Test("conforms to Equatable")
    func equatableConformance() {
        let a = Quota.watchZones
        let b = Quota.watchZones
        #expect(a == b)
    }

    @Test("conforms to Hashable")
    func hashableConformance() {
        let set: Set<Quota> = [.watchZones, .watchZones]
        #expect(set.count == 1)
    }
}
