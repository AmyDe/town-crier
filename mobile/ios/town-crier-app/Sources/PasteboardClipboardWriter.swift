import TownCrierDomain
import UIKit

struct PasteboardClipboardWriter: ClipboardWriter {
  func copy(_ text: String) {
    UIPasteboard.general.string = text
  }
}
