import Foundation
import Network
import TownCrierDomain

/// Monitors network connectivity using NWPathMonitor.
public final class NWPathConnectivityMonitor: ConnectivityMonitor, @unchecked Sendable {
  private let monitor: NWPathMonitor
  private let queue: DispatchQueue
  private var _isConnected: Bool = true

  public var isConnected: Bool { _isConnected }

  public init() {
    monitor = NWPathMonitor()
    queue = DispatchQueue(label: "town-crier.connectivity")
    monitor.pathUpdateHandler = { [weak self] path in
      self?._isConnected = path.status == .satisfied
    }
    monitor.start(queue: queue)
  }

  deinit {
    monitor.cancel()
  }
}
