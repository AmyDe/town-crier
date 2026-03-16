import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("MetricKitCrashReporter")
struct MetricKitCrashReporterTests {
    @Test func conformsToCrashReporter() {
        let sut: any CrashReporter = MetricKitCrashReporter()
        #expect(sut is MetricKitCrashReporter)
    }

    @Test func start_doesNotThrow() {
        let sut = MetricKitCrashReporter()
        sut.start()
        // No assertion needed — verifying start() can be called without error
    }
}
