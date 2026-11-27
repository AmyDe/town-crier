import Foundation
import TownCrierDomain

extension Webhook {
  static func fixture(
    url: String = "https://example.com/hook",
    format: WebhookFormat? = .json,
    status: WebhookStatus = .active,
    disabledReason: WebhookDisabledReason? = nil,
    paused: Bool = false,
    createdAt: Date = Date(timeIntervalSince1970: 1_795_701_600),
    lastSuccessAt: Date? = nil,
    lastFailureAt: Date? = nil,
    lastFailureReason: WebhookFailureReason? = nil,
    lastFailureStatus: Int? = nil
  ) -> Webhook {
    Webhook(
      url: url,
      format: format,
      status: status,
      disabledReason: disabledReason,
      isPaused: paused,
      createdAt: createdAt,
      lastSuccessAt: lastSuccessAt,
      lastFailureAt: lastFailureAt,
      lastFailureReason: lastFailureReason,
      lastFailureStatus: lastFailureStatus
    )
  }
}
