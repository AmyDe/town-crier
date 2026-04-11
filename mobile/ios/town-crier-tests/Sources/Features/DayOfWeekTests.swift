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
}
