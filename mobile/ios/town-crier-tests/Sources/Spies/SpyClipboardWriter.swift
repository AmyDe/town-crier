import TownCrierDomain

final class SpyClipboardWriter: ClipboardWriter, @unchecked Sendable {
  private(set) var copyCalls: [String] = []

  func copy(_ text: String) {
    copyCalls.append(text)
  }
}
