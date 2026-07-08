import Foundation
import TownCrierDomain

final class SpyDeviceLocalZoneRepository: DeviceLocalZoneRepository, @unchecked Sendable {
  var loadAllResult: [DeviceLocalZone] = []
  private(set) var loadAllCallCount = 0

  func loadAll() -> [DeviceLocalZone] {
    loadAllCallCount += 1
    return loadAllResult
  }

  private(set) var saveCalls: [DeviceLocalZone] = []
  var saveError: Error?

  func save(_ zone: DeviceLocalZone) throws {
    saveCalls.append(zone)
    if let saveError {
      throw saveError
    }
  }

  private(set) var deleteCalls: [DeviceLocalZoneId] = []

  func delete(_ id: DeviceLocalZoneId) {
    deleteCalls.append(id)
  }

  var activeZoneIdResult: DeviceLocalZoneId?
  private(set) var setActiveZoneIdCalls: [DeviceLocalZoneId?] = []

  func activeZoneId() -> DeviceLocalZoneId? {
    activeZoneIdResult
  }

  func setActiveZoneId(_ id: DeviceLocalZoneId?) {
    setActiveZoneIdCalls.append(id)
    activeZoneIdResult = id
  }
}
