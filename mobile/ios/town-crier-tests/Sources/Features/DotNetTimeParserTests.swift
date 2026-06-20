import Foundation
import Testing

@testable import TownCrierData

/// Tests for the shared `DotNetTimeParser` that decodes the Go backend's
/// `platform.DotNetTime` wire format (`2006-01-02T15:04:05.9999999-07:00`).
/// Fractional seconds appear only when the sub-second part is non-zero, so a
/// robust parser must accept both fractional and whole-second strings, plus the
/// `Z` zero-offset spelling.
@Suite("DotNetTimeParser")
struct DotNetTimeParserTests {

  @Test("parses a fractional-seconds timestamp")
  func parsesFractionalSeconds() throws {
    let parsed = try #require(
      DotNetTimeParser.date(from: "2026-07-20T14:23:45.6789123+00:00")
    )
    // Truncated to whole seconds plus the fractional remainder; assert the
    // whole-second instant is correct and the fraction is carried.
    let expectedWholeSecond = try #require(
      DotNetTimeParser.date(from: "2026-07-20T14:23:45+00:00")
    )
    #expect(parsed.timeIntervalSince(expectedWholeSecond) > 0.6)
    #expect(parsed.timeIntervalSince(expectedWholeSecond) < 0.7)
  }

  @Test("parses a whole-second timestamp with explicit offset")
  func parsesWholeSecondWithOffset() throws {
    let parsed = try #require(
      DotNetTimeParser.date(from: "2026-06-12T09:30:00+00:00")
    )
    let reference = ISO8601DateFormatter()
    reference.formatOptions = [.withInternetDateTime]
    let expected = try #require(reference.date(from: "2026-06-12T09:30:00+00:00"))
    #expect(parsed == expected)
  }

  @Test("parses a whole-second timestamp in Z form")
  func parsesWholeSecondZForm() throws {
    let parsed = try #require(
      DotNetTimeParser.date(from: "2026-06-12T09:30:00Z")
    )
    let reference = ISO8601DateFormatter()
    reference.formatOptions = [.withInternetDateTime]
    let expected = try #require(reference.date(from: "2026-06-12T09:30:00Z"))
    #expect(parsed == expected)
  }

  @Test("parses a fractional-seconds timestamp in Z form")
  func parsesFractionalSecondsZForm() throws {
    let parsed = try #require(
      DotNetTimeParser.date(from: "2026-07-20T14:23:45.5Z")
    )
    let expectedWholeSecond = try #require(
      DotNetTimeParser.date(from: "2026-07-20T14:23:45Z")
    )
    #expect(parsed.timeIntervalSince(expectedWholeSecond) > 0.4)
    #expect(parsed.timeIntervalSince(expectedWholeSecond) < 0.6)
  }

  @Test("returns nil on genuine garbage")
  func returnsNilOnGarbage() {
    #expect(DotNetTimeParser.date(from: "not a date") == nil)
    #expect(DotNetTimeParser.date(from: "") == nil)
    #expect(DotNetTimeParser.date(from: "2026-13-45") == nil)
  }
}
