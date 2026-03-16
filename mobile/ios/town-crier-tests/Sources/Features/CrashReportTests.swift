import Foundation
import Testing

@testable import TownCrierDomain

@Suite("CrashReport")
struct CrashReportTests {
    @Test func init_storesAllProperties() {
        let timestamp = Date(timeIntervalSince1970: 1_700_000_000)
        let report = CrashReport(
            id: "crash-001",
            timestamp: timestamp,
            signal: "SIGSEGV",
            reason: "Segmentation fault",
            terminationDescription: "Namespace SIGNAL, Code 11"
        )

        #expect(report.id == "crash-001")
        #expect(report.timestamp == timestamp)
        #expect(report.signal == "SIGSEGV")
        #expect(report.reason == "Segmentation fault")
        #expect(report.terminationDescription == "Namespace SIGNAL, Code 11")
    }

    @Test func equality_sameProperties_areEqual() {
        let timestamp = Date(timeIntervalSince1970: 1_700_000_000)
        let report1 = CrashReport(
            id: "crash-001",
            timestamp: timestamp,
            signal: "SIGSEGV",
            reason: "Segmentation fault",
            terminationDescription: "Namespace SIGNAL, Code 11"
        )
        let report2 = CrashReport(
            id: "crash-001",
            timestamp: timestamp,
            signal: "SIGSEGV",
            reason: "Segmentation fault",
            terminationDescription: "Namespace SIGNAL, Code 11"
        )

        #expect(report1 == report2)
    }
}
