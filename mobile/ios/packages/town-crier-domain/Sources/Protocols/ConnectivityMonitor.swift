/// Port for checking network connectivity status.
public protocol ConnectivityMonitor: Sendable {
    var isConnected: Bool { get }
}
