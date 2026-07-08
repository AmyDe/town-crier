import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("UserDefaultsAnonymousBrowseStateRepository")
struct UserDefaultsAnonymousBrowseStateRepositoryTests {
  private func makeSUT() -> UserDefaultsAnonymousBrowseStateRepository {
    // Isolated suite per test so parallel test runs never share state.
    let defaults = UserDefaults(suiteName: UUID().uuidString)
    // swiftlint:disable:next force_unwrapping
    return UserDefaultsAnonymousBrowseStateRepository(defaults: defaults!)
  }

  @Test("load returns nil when nothing has been saved")
  func load_returnsNil_whenNothingSaved() {
    let sut = makeSUT()

    #expect(sut.load() == nil)
  }

  @Test("save then load round-trips the exact state")
  func saveThenLoad_roundTrips() throws {
    let sut = makeSUT()
    let postcode = try Postcode("CB1 2AD")
    let coordinate = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    let createdAt = Date(timeIntervalSince1970: 1_700_000_000)
    let state = AnonymousBrowseState(
      postcode: postcode, coordinate: coordinate, createdAt: createdAt)

    sut.save(state)

    #expect(sut.load() == state)
  }

  @Test("save replaces any previously saved state")
  func save_replacesPreviousState() throws {
    let sut = makeSUT()
    let first = AnonymousBrowseState(
      postcode: try Postcode("CB1 2AD"),
      coordinate: try Coordinate(latitude: 52.2053, longitude: 0.1218),
      createdAt: Date(timeIntervalSince1970: 1_700_000_000)
    )
    let second = AnonymousBrowseState(
      postcode: try Postcode("SW1A 1AA"),
      coordinate: try Coordinate(latitude: 51.5014, longitude: -0.1419),
      createdAt: Date(timeIntervalSince1970: 1_800_000_000)
    )
    sut.save(first)

    sut.save(second)

    #expect(sut.load() == second)
  }

  @Test("clear removes any saved state")
  func clear_removesSavedState() throws {
    let sut = makeSUT()
    let state = AnonymousBrowseState(
      postcode: try Postcode("CB1 2AD"),
      coordinate: try Coordinate(latitude: 52.2053, longitude: 0.1218),
      createdAt: Date(timeIntervalSince1970: 1_700_000_000)
    )
    sut.save(state)

    sut.clear()

    #expect(sut.load() == nil)
  }

  @Test("clear is a no-op when nothing was saved")
  func clear_noop_whenNothingSaved() {
    let sut = makeSUT()

    sut.clear()

    #expect(sut.load() == nil)
  }
}
