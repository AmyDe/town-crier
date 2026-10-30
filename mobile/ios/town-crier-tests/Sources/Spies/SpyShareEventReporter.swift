import TownCrierDomain

final class SpyShareEventReporter: ShareEventReporter, @unchecked Sendable {
  private(set) var reportCalls: [ShareDestination] = []

  func report(_ destination: ShareDestination) async {
    reportCalls.append(destination)
  }
}
