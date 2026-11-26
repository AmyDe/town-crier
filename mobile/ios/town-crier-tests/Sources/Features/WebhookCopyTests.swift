import Foundation
import Testing

@testable import TownCrierPresentation

@Suite("WebhookCopy")
struct WebhookCopyTests {
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
