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
        == "Send your alerts to your own URL, as signed JSON, a Slack message or a Discord message.")
    #expect(WebhookCopy.errorFormatInvalid == "Choose JSON, Slack or Discord.")
    #expect(DomainError.webhookFormatInvalid.userMessage == "Choose JSON, Slack or Discord.")
  }

  @Test("the upsell pitch uses the agreed copy")
  func upsell_usesAgreedCopy() {
    #expect(WebhookCopy.upsellHeading == "Send your alerts to Slack, Discord or your own system")
    #expect(
      WebhookCopy.upsellBody
        == "Your planning alerts post to a channel your whole team already reads. "
        + "No one else needs the app.")
    #expect(WebhookCopy.upsellExampleTitle == "Town Crier: 3 planning updates")
    #expect(
      WebhookCopy.upsellExampleLines == [
        "\u{2022} New near Home: 26/01234/FUL, 1 High Street, Leeds. Single-storey rear extension",
        "\u{2022} Decision (Permitted): 26/01001/HOU, 4 Mill Lane, Leeds. Loft conversion",
        "\u{2022} New near Office: 26/09999/FUL, 9 Park Row, Leeds. Change of use to caf\u{00E9}",
      ])
    #expect(WebhookCopy.upsellExampleLabel == "Example Slack message")
    #expect(WebhookCopy.upsellFootnote == "Part of Pro, with unlimited watch zones up to 10 km.")
    #expect(WebhookCopy.seeProPlans == "See Pro plans")
    #expect(WebhookCopy.docsLink == "Read the webhook docs")
  }

  @Test("the upsell copy never mentions how often alerts are checked")
  func upsell_hasNoCadenceWording() {
    let upsell =
      [
        WebhookCopy.upsellHeading, WebhookCopy.upsellBody, WebhookCopy.upsellExampleTitle,
        WebhookCopy.upsellExampleLabel, WebhookCopy.upsellFootnote, WebhookCopy.seeProPlans,
      ] + WebhookCopy.upsellExampleLines
    #expect(upsell.allSatisfy { !$0.lowercased().contains("hour") })
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
