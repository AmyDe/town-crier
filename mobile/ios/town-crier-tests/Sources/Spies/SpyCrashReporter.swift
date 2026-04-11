import TownCrierDomain

final class SpyCrashReporter: CrashReporter, @unchecked Sendable {
  nonisolated(unsafe) private(set) var startCallCount = 0

  func start() {
    startCallCount += 1
  }
}
