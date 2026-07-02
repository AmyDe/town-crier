import Foundation
import TownCrierPresentation
import os

/// TEMPORARY (tc-28x2, GH #763 Problem 1): logs which of the two inbound
/// Universal Link delivery paths — SwiftUI's `.onContinueUserActivity`
/// (`UniversalLinkModifier`) or the `AppDelegate` fallback — actually fires
/// on-device. Confirms the next TestFlight build's routing; remove once
/// confirmed.
enum UniversalLinkDeliveryLogger {
  private static let logger = Logger(subsystem: "uk.towncrierapp", category: "UniversalLink")

  static func logDelivery(source: String, url: URL?, deepLink: DeepLink) {
    logger.notice(
      "UL via \(source): \(url?.absoluteString ?? "nil") -> \(String(describing: deepLink))")
  }
}
