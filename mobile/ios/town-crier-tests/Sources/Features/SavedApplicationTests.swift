import Foundation
import Testing
import TownCrierDomain

@Suite("SavedApplication")
struct SavedApplicationTests {

  @Test("init stores applicationUid and savedAt")
  func init_storesProperties() {
    let date = Date(timeIntervalSince1970: 1_700_000_000)
    let saved = SavedApplication(
      applicationUid: "BK/2026/0042",
      savedAt: date,
      application: nil
    )

    #expect(saved.applicationUid == "BK/2026/0042")
    #expect(saved.savedAt == date)
    #expect(saved.application == nil)
  }

  @Test("init stores nested application when provided")
  func init_storesNestedApplication() {
    let app = PlanningApplication(
      id: PlanningApplicationId("APP-001"),
      reference: ApplicationReference("2026/0042"),
      authority: LocalAuthority(code: "CAM", name: "Cambridge"),
      status: .undecided,
      receivedDate: Date(timeIntervalSince1970: 1_700_000_000),
      description: "Rear extension",
      address: "12 Mill Road"
    )
    let saved = SavedApplication(
      applicationUid: "APP-001",
      savedAt: Date(timeIntervalSince1970: 1_700_100_000),
      application: app
    )

    #expect(saved.application == app)
  }

  @Test("two SavedApplications with same values are equal")
  func equatable_sameValues_areEqual() {
    let date = Date(timeIntervalSince1970: 1_700_000_000)
    let a = SavedApplication(applicationUid: "UID-1", savedAt: date, application: nil)
    let b = SavedApplication(applicationUid: "UID-1", savedAt: date, application: nil)

    #expect(a == b)
  }

  @Test("two SavedApplications with different UIDs are not equal")
  func equatable_differentUids_areNotEqual() {
    let date = Date(timeIntervalSince1970: 1_700_000_000)
    let a = SavedApplication(applicationUid: "UID-1", savedAt: date, application: nil)
    let b = SavedApplication(applicationUid: "UID-2", savedAt: date, application: nil)

    #expect(a != b)
  }
}
