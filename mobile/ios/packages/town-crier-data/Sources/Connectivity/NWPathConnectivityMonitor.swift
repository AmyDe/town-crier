import Foundation
import Network
import os
import TownCrierDomain

/// Monitors network connectivity using NWPathMonitor.
public final class NWPathConnectivityMonitor: ConnectivityMonitor {
  private let monitor: NWPathMonitor
  private let queue: DispatchQueue
  private let connected = OSAllocatedUnfairLock(initialState: true)

  public var isConnected: Bool { connected.withLock { $0 } }

  public init() {
    monitor = NWPathMonitor()
    queue = DispatchQueue(label: "town-crier.connectivity")
    monitor.pathUpdateHandler = { [weak self] path in
      let satisfied = path.status == .satisfied
      self?.connected.withLock { $0 = satisfied }
    }
    monitor.start(queue: queue)
  }

  deinit {
    monitor.cancel()
  }
}
