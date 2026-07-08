import Foundation
import Testing
import TownCrierDomain

@Suite("AnonymousBrowseState")
struct AnonymousBrowseStateTests {
  @Test("stores postcode, coordinate, and createdAt as provided")
  func init_storesProvidedValues() throws {
    let postcode = try Postcode("CB1 2AD")
    let coordinate = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    let createdAt = Date(timeIntervalSince1970: 1_700_000_000)

    let sut = AnonymousBrowseState(postcode: postcode, coordinate: coordinate, createdAt: createdAt)

    #expect(sut.postcode == postcode)
    #expect(sut.coordinate == coordinate)
    #expect(sut.createdAt == createdAt)
  }

  @Test("two states with identical fields are equal")
  func equatable_matchesOnAllFields() throws {
    let postcode = try Postcode("CB1 2AD")
    let coordinate = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    let createdAt = Date(timeIntervalSince1970: 1_700_000_000)

    let first = AnonymousBrowseState(postcode: postcode, coordinate: coordinate, createdAt: createdAt)
    let second = AnonymousBrowseState(postcode: postcode, coordinate: coordinate, createdAt: createdAt)

    #expect(first == second)
  }
}
