import Foundation
import MetricKit
import TownCrierDomain
import os

/// Crash reporter that uses MetricKit to receive crash diagnostics from the system.
///
/// MetricKit delivers crash diagnostic payloads up to 24 hours after the crash occurs.
/// Call `start()` once during app launch to subscribe to diagnostics.
public final class MetricKitCrashReporter: NSObject, CrashReporter, MXMetricManagerSubscriber,
  @unchecked Sendable
{
  private let logger = Logger(
    subsystem: "uk.co.towncrier",
    category: "CrashReporting"
  )

  public override init() {
    super.init()
  }

  public func start() {
    MXMetricManager.shared.add(self)
    logger.info("MetricKit crash reporter started")
  }

  // MARK: - MXMetricManagerSubscriber

  public func didReceive(_ payloads: [MXDiagnosticPayload]) {
    for payload in payloads {
      if let crashDiagnostics = payload.crashDiagnostics {
        for diagnostic in crashDiagnostics {
          let report = CrashReport(
            id: UUID().uuidString,
            timestamp: payload.timeStampEnd,
            signal: diagnostic.signal?.description ?? "Unknown",
            reason: diagnostic.terminationReason ?? "Unknown",
            terminationDescription: diagnostic.virtualMemoryRegionInfo ?? ""
          )
          logger.error(
            "Crash received: signal=\(report.signal) reason=\(report.reason)")
        }
      }
    }
  }
}
