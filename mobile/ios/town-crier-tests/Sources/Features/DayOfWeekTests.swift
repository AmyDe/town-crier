import Foundation
import Testing
import TownCrierDomain

@Suite("DayOfWeek")
struct DayOfWeekTests {

  @Test("all seven days are defined")
  func allSevenDays() {
    let allDays = DayOfWeek.allCases
    #expect(allDays.count == 7)
  }

  @Test("raw values match API strings")
  func rawValuesMatchAPI() {
    #expect(DayOfWeek.sunday.rawValue == "Sunday")
    #expect(DayOfWeek.monday.rawValue == "Monday")
    #expect(DayOfWeek.tuesday.rawValue == "Tuesday")
    #expect(DayOfWeek.wednesday.rawValue == "Wednesday")
    #expect(DayOfWeek.thursday.rawValue == "Thursday")
    #expect(DayOfWeek.friday.rawValue == "Friday")
    #expect(DayOfWeek.saturday.rawValue == "Saturday")
  }

  @Test("decodes from API JSON string")
  func decodesFromJSON() throws {
    let json = Data(#""Monday""#.utf8)
    let decoded = try JSONDecoder().decode(DayOfWeek.self, from: json)
    #expect(decoded == .monday)
  }

  @Test("encodes to API JSON string")
  func encodesToJSON() throws {
    let encoded = try JSONEncoder().encode(DayOfWeek.friday)
    let string = String(data: encoded, encoding: .utf8)
    #expect(string == #""Friday""#)
  }

  @Test("displayName is the human-readable form for each day")
  func displayNameMatchesEachDay() {
    #expect(DayOfWeek.sunday.displayName == "Sunday")
    #expect(DayOfWeek.monday.displayName == "Monday")
    #expect(DayOfWeek.tuesday.displayName == "Tuesday")
    #expect(DayOfWeek.wednesday.displayName == "Wednesday")
    #expect(DayOfWeek.thursday.displayName == "Thursday")
    #expect(DayOfWeek.friday.displayName == "Friday")
    #expect(DayOfWeek.saturday.displayName == "Saturday")
  }

  // MARK: - UK display order (Monday-first)

  @Test("weekOrderUK starts on Monday and ends on Sunday")
  func weekOrderUKIsMondayFirst() {
    #expect(
      DayOfWeek.weekOrderUK == [
        .monday, .tuesday, .wednesday, .thursday, .friday, .saturday, .sunday,
      ])
    #expect(DayOfWeek.weekOrderUK.first == .monday)
    #expect(DayOfWeek.weekOrderUK.last == .sunday)
  }

  @Test("weekOrderUK contains all seven days with no duplicates")
  func weekOrderUKCoversEveryDayOnce() {
    #expect(DayOfWeek.weekOrderUK.count == 7)
    #expect(Set(DayOfWeek.weekOrderUK) == Set(DayOfWeek.allCases))
  }

  // MARK: - Wire-contract regression guards

  @Test("allCases declaration order is unchanged (Sunday-first wire order)")
  func allCasesStillSundayFirst() {
    #expect(DayOfWeek.allCases.first == .sunday)
    #expect(DayOfWeek.allCases.last == .saturday)
  }

  @Test("rawValue is the English day name for every case")
  func rawValueIsEnglishDayName() {
    #expect(DayOfWeek.monday.rawValue == "Monday")
    #expect(DayOfWeek.sunday.rawValue == "Sunday")
    #expect(DayOfWeek.saturday.rawValue == "Saturday")
  }
}
