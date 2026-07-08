import Foundation
import TownCrierDomain

final class SpyAnonymousBrowseStateRepository: AnonymousBrowseStateRepository, @unchecked Sendable {
  var loadResult: AnonymousBrowseState?
  private(set) var loadCallCount = 0

  func load() -> AnonymousBrowseState? {
    loadCallCount += 1
    return loadResult
  }

  private(set) var saveCalls: [AnonymousBrowseState] = []

  func save(_ state: AnonymousBrowseState) {
    saveCalls.append(state)
    loadResult = state
  }

  private(set) var clearCallCount = 0

  func clear() {
    clearCallCount += 1
    loadResult = nil
  }
}
