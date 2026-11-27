import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("WebhookCopy")
struct WebhookCopyTests {
  @Test("the picker offers JSON, Slack and Discord")
  func formats_areNamed() {
    #expect(WebhookFormat.allCases.map { WebhookCopy.formatName($0) } == ["JSON", "Slack", "Discord"])
  }

  @Test("a format this app does not know is shown as Other")
  func formatName_unknown_isOther() {
    #expect(WebhookCopy.formatName(nil) == "Other")
  }

  @Test("the intro and the format error name all three formats")
  func copy_namesAllFormats() {
    #expect(
      WebhookCopy.intro
        == "Send your alerts to your own URL, as signed JSON, a Slack message or a Discord message. "
        + "They go out at the end of each hourly check.")
    #expect(WebhookCopy.errorFormatInvalid == "Choose JSON, Slack or Discord.")
    #expect(DomainError.webhookFormatInvalid.userMessage == "Choose JSON, Slack or Discord.")
  }

  @Test("no string in WebhookCopy contains an em dash")
  func source_hasNoEmDash() throws {
    let source = URL(fileURLWithPath: #filePath)
      .deletingLastPathComponent()
      .appendingPathComponent(
        "../../../packages/town-crier-presentation/Sources/Features/Webhooks/WebhookCopy.swift")
      .standardized

    let contents = try String(contentsOf: source, encoding: .utf8)

    #expect(contents.contains("enum WebhookCopy"))
    #expect(!contents.contains("\u{2014}"))
  }
}
