import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("ShareLinkCode")
struct ShareLinkCodeTests {
  @Test func generate_isSixAlphanumericCharacters() {
    for _ in 0..<200 {
      let code = ShareLinkCode.generate()
      #expect(code.wholeMatch(of: /[A-Za-z0-9]{6}/) != nil)
    }
  }

  @Test func generate_isFreshEachCall() {
    let codes = Set((0..<200).map { _ in ShareLinkCode.generate() })
    #expect(codes.count == 200)
  }

  @Test(arguments: ["abc", "abc1234", "abc-12", "abc12é", "", "abc12\n"])
  func validated_rejects(value: String) {
    #expect(ShareLinkCode.validated(value) == nil)
  }

  @Test func validated_acceptsAValidCode() {
    #expect(ShareLinkCode.validated("aZ09bY") == "aZ09bY")
  }

  @Test func shareURL_appendsTheCode() {
    let url = ShareURL.build(authoritySlug: "croydon", ref: "23/03456/FUL", linkCode: "aZ09bY")

    #expect(url?.absoluteString == "https://share.towncrierapp.uk/a/croydon/23/03456/FUL?s=aZ09bY")
  }
}

@Suite("UniversalLinkParser link code")
struct UniversalLinkParserLinkCodeTests {
  private func parse(_ query: String) throws -> DeepLink? {
    UniversalLinkParser.parse(
      try #require(URL(string: "https://share.towncrierapp.uk/a/croydon/23/03456/FUL\(query)")))
  }

  @Test func keepsAValidCode() throws {
    #expect(
      try parse("?s=aZ09bY")
        == .shareApplication(authoritySlug: "croydon", ref: "23/03456/FUL", linkCode: "aZ09bY"))
  }

  @Test(arguments: ["", "?s=abc", "?s=abc1234", "?s=abc-12", "?t=aZ09bY"])
  func dropsAnInvalidCodeAndKeepsTheLink(query: String) throws {
    #expect(try parse(query) == .shareApplication(authoritySlug: "croydon", ref: "23/03456/FUL"))
  }
}

@Suite("ShareDestination")
struct ShareDestinationTests {
  @Test(
    arguments: [
      ("com.apple.UIKit.activity.Message", ShareDestination.messages),
      ("com.apple.UIKit.activity.Mail", .mail),
      ("net.whatsapp.WhatsApp.ShareExtension", .whatsapp),
      ("com.facebook.Facebook.ShareExtension", .facebook),
      ("com.apple.UIKit.activity.CopyToPasteboard", .copy),
      ("com.example.Unknown", .other),
    ] as [(String, ShareDestination)])
  func mapsActivityType(activityType: String, destination: ShareDestination) {
    #expect(ShareDestination(activityType: activityType) == destination)
  }

  @Test func nilActivityTypeIsOther() {
    #expect(ShareDestination(activityType: nil) == .other)
  }

  @Test func wireValuesMatchTheClosedSet() {
    #expect(
      ShareDestination.allCases.map(\.rawValue).sorted()
        == ["copy", "facebook", "mail", "messages", "other", "whatsapp"])
  }
}
