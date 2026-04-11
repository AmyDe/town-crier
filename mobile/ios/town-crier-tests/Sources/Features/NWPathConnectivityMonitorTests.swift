import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("NWPathConnectivityMonitor")
struct NWPathConnectivityMonitorTests {
  @Test func init_defaultsToConnected() {
    let sut = NWPathConnectivityMonitor()

    // Before any NWPathMonitor update, we assume connected
    // to avoid false offline on app launch
    #expect(sut.isConnected)
  }
}
