import TownCrierDomain

final class StubConnectivityMonitor: ConnectivityMonitor, @unchecked Sendable {
  var isConnected: Bool

  init(isConnected: Bool = true) {
    self.isConnected = isConnected
  }
}
