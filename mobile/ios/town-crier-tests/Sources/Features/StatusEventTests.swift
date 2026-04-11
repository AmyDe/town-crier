import Foundation
import Testing
import TownCrierDomain

@Suite("StatusEvent")
struct StatusEventTests {

  @Test func init_capturesStatusAndDate() {
    let date = Date(timeIntervalSince1970: 1_700_000_000)
    let event = StatusEvent(status: .undecided, date: date)

    #expect(event.status == .undecided)
    #expect(event.date == date)
  }

  @Test func equality_sameValues_areEqual() {
    let date = Date(timeIntervalSince1970: 1_700_000_000)
    let event1 = StatusEvent(status: .approved, date: date)
    let event2 = StatusEvent(status: .approved, date: date)

    #expect(event1 == event2)
  }

  @Test func equality_differentStatus_areNotEqual() {
    let date = Date(timeIntervalSince1970: 1_700_000_000)
    let event1 = StatusEvent(status: .approved, date: date)
    let event2 = StatusEvent(status: .refused, date: date)

    #expect(event1 != event2)
  }

  @Test func chronologicalOrder_sortsOldestFirst() {
    let early = StatusEvent(status: .undecided, date: Date(timeIntervalSince1970: 1_700_000_000))
    let late = StatusEvent(status: .approved, date: Date(timeIntervalSince1970: 1_700_100_000))
    let events = [late, early].sorted()

    #expect(events == [early, late])
  }
}
