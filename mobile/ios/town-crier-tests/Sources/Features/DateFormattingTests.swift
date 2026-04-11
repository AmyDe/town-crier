import Foundation
import Testing

@testable import TownCrierPresentation

@Suite("Date+TownCrier")
struct DateFormattingTests {

  @Test func formattedForDisplay_producesUKFormat() {
    // 1_700_000_000 = 14 Nov 2023 in UTC
    let date = Date(timeIntervalSince1970: 1_700_000_000)

    #expect(date.formattedForDisplay == "14 Nov 2023")
  }

  @Test func formattedForDisplay_firstJanuary_showsSingleDigitDay() {
    // 1 Jan 2024 00:00:00 UTC = 1704067200
    let date = Date(timeIntervalSince1970: 1_704_067_200)

    #expect(date.formattedForDisplay == "1 Jan 2024")
  }

  @Test func formattedForDisplay_usesUTCNotLocalTimezone() {
    // 31 Dec 2023 23:30:00 UTC = 1704065400
    // This is already 1 Jan 2024 in UTC+1 timezones, but should show 31 Dec in UTC
    let date = Date(timeIntervalSince1970: 1_704_065_400)

    #expect(date.formattedForDisplay == "31 Dec 2023")
  }
}
