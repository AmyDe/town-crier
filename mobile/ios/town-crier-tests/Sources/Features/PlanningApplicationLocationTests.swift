import Foundation
import Testing

@testable import TownCrierDomain

@Suite("PlanningApplication location")
struct PlanningApplicationLocationTests {
  @Test func location_canBeNil() {
    let app = PlanningApplication.pendingReview
    // pendingReview fixture should have a location set for map testing
    // but the field should be optional on the type
    let noLocation = PlanningApplication(
      id: PlanningApplicationId("APP-NO-LOC"),
      reference: ApplicationReference("2026/0001"),
      authority: LocalAuthority(code: "CAM", name: "Cambridge"),
      status: .underReview,
      receivedDate: Date(timeIntervalSince1970: 1_700_000_000),
      description: "No location app",
      address: "Unknown Address",
      location: nil
    )
    #expect(noLocation.location == nil)
  }

  @Test func location_canBeSet() throws {
    let coordinate = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    let app = PlanningApplication(
      id: PlanningApplicationId("APP-LOC"),
      reference: ApplicationReference("2026/0002"),
      authority: LocalAuthority(code: "CAM", name: "Cambridge"),
      status: .underReview,
      receivedDate: Date(timeIntervalSince1970: 1_700_000_000),
      description: "Located app",
      address: "12 Mill Road, Cambridge",
      location: coordinate
    )
    #expect(app.location == coordinate)
  }

  @Test func fixtures_pendingReview_hasLocation() {
    #expect(PlanningApplication.pendingReview.location != nil)
  }

  @Test func fixtures_approved_hasLocation() {
    #expect(PlanningApplication.approved.location != nil)
  }
}
