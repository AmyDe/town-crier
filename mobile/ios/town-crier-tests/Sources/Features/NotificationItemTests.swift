import Foundation
import Testing
import TownCrierDomain

@Suite("NotificationItem")
struct NotificationItemTests {

  @Test("initialises with all fields")
  func initialisesWithAllFields() {
    let date = Date(timeIntervalSince1970: 1_712_000_000)
    let item = NotificationItem(
      applicationName: "Rear extension at 12 Mill Road",
      applicationAddress: "12 Mill Road, Cambridge, CB1 2AD",
      applicationDescription: "Erection of two-storey rear extension",
      applicationType: "Full Planning Application",
      authorityId: 123,
      createdAt: date,
      eventType: "DecisionUpdate",
      decision: "Permitted",
      sources: "Zone, Saved"
    )

    #expect(item.applicationName == "Rear extension at 12 Mill Road")
    #expect(item.applicationAddress == "12 Mill Road, Cambridge, CB1 2AD")
    #expect(item.applicationDescription == "Erection of two-storey rear extension")
    #expect(item.applicationType == "Full Planning Application")
    #expect(item.authorityId == 123)
    #expect(item.createdAt == date)
    #expect(item.eventType == "DecisionUpdate")
    #expect(item.decision == "Permitted")
    #expect(item.sources == "Zone, Saved")
  }

  @Test("decision is optional")
  func decisionIsOptional() {
    let item = NotificationItem(
      applicationName: "New application",
      applicationAddress: "Address",
      applicationDescription: "Description",
      applicationType: "Type",
      authorityId: 1,
      createdAt: Date(timeIntervalSince1970: 1_712_000_000),
      eventType: "NewApplication",
      decision: nil,
      sources: "Zone"
    )

    #expect(item.decision == nil)
    #expect(item.eventType == "NewApplication")
  }

  @Test("conforms to Equatable")
  func conformsToEquatable() {
    let date = Date(timeIntervalSince1970: 1_712_000_000)
    let itemA = NotificationItem(
      applicationName: "Test",
      applicationAddress: "Address",
      applicationDescription: "Description",
      applicationType: "Type",
      authorityId: 1,
      createdAt: date,
      eventType: "NewApplication",
      decision: nil,
      sources: "Zone"
    )
    let itemB = NotificationItem(
      applicationName: "Test",
      applicationAddress: "Address",
      applicationDescription: "Description",
      applicationType: "Type",
      authorityId: 1,
      createdAt: date,
      eventType: "NewApplication",
      decision: nil,
      sources: "Zone"
    )

    #expect(itemA == itemB)
  }

  @Test("conforms to Sendable")
  func conformsToSendable() {
    let date = Date(timeIntervalSince1970: 1_712_000_000)
    let item = NotificationItem(
      applicationName: "Test",
      applicationAddress: "Address",
      applicationDescription: "Description",
      applicationType: "Type",
      authorityId: 1,
      createdAt: date,
      eventType: "NewApplication",
      decision: nil,
      sources: "Zone"
    )
    let sendable: any Sendable = item
    #expect(sendable is NotificationItem)
  }
}

@Suite("NotificationPage")
struct NotificationPageTests {

  private static let date = Date(timeIntervalSince1970: 1_712_000_000)

  private static func makeItem(name: String = "Test") -> NotificationItem {
    NotificationItem(
      applicationName: name,
      applicationAddress: "Address",
      applicationDescription: "Description",
      applicationType: "Type",
      authorityId: 1,
      createdAt: date,
      eventType: "NewApplication",
      decision: nil,
      sources: "Zone"
    )
  }

  @Test("initialises with notifications, total, and page")
  func initialisesWithAllFields() {
    let items = [Self.makeItem(name: "A"), Self.makeItem(name: "B")]
    let page = NotificationPage(notifications: items, total: 10, page: 1)

    #expect(page.notifications.count == 2)
    #expect(page.total == 10)
    #expect(page.page == 1)
  }

  @Test("hasMore returns true when more pages exist")
  func hasMore_morePages_returnsTrue() {
    let items = [Self.makeItem(), Self.makeItem()]
    let page = NotificationPage(notifications: items, total: 10, page: 1)

    #expect(page.hasMore)
  }

  @Test("hasMore returns false when all items are loaded")
  func hasMore_allLoaded_returnsFalse() {
    let items = [Self.makeItem(), Self.makeItem()]
    let page = NotificationPage(notifications: items, total: 2, page: 1)

    #expect(!page.hasMore)
  }

  @Test("hasMore returns false when notifications are empty")
  func hasMore_empty_returnsFalse() {
    let page = NotificationPage(notifications: [], total: 0, page: 1)

    #expect(!page.hasMore)
  }

  @Test("conforms to Equatable")
  func conformsToEquatable() {
    let items = [Self.makeItem()]
    let pageA = NotificationPage(notifications: items, total: 1, page: 1)
    let pageB = NotificationPage(notifications: items, total: 1, page: 1)

    #expect(pageA == pageB)
  }

  @Test("conforms to Sendable")
  func conformsToSendable() {
    let page = NotificationPage(notifications: [], total: 0, page: 1)
    let sendable: any Sendable = page
    #expect(sendable is NotificationPage)
  }
}
