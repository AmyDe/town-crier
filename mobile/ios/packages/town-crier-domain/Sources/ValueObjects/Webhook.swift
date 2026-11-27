import Foundation

/// The user's outbound webhook for alerts.
public struct Webhook: Equatable, Sendable {
  public let url: String
  /// `nil` means a format this app version does not know.
  public let format: WebhookFormat?
  public let status: WebhookStatus
  /// Why the server switched the webhook off. `nil` when unknown or not switched off.
  public let disabledReason: WebhookDisabledReason?
  /// `true` when the user is not on Pro, so no alerts are sent.
  public let isPaused: Bool
  public let createdAt: Date
  public let lastSuccessAt: Date?
  public let lastFailureAt: Date?
  /// `nil` when there is no failure, or the reason is not one this client knows.
  public let lastFailureReason: WebhookFailureReason?
  public let lastFailureStatus: Int?

  public init(
    url: String,
    format: WebhookFormat?,
    status: WebhookStatus,
    disabledReason: WebhookDisabledReason?,
    isPaused: Bool,
    createdAt: Date,
    lastSuccessAt: Date?,
    lastFailureAt: Date?,
    lastFailureReason: WebhookFailureReason?,
    lastFailureStatus: Int?
  ) {
    self.url = url
    self.format = format
    self.status = status
    self.disabledReason = disabledReason
    self.isPaused = isPaused
    self.createdAt = createdAt
    self.lastSuccessAt = lastSuccessAt
    self.lastFailureAt = lastFailureAt
    self.lastFailureReason = lastFailureReason
    self.lastFailureStatus = lastFailureStatus
  }
}

/// The body a webhook receives. Raw values are the wire values.
public enum WebhookFormat: String, CaseIterable, Sendable {
  case json
  case slack
  case discord
}

/// Whether the server delivers to the webhook. Raw values are the wire values.
public enum WebhookStatus: String, Sendable {
  case active
  case disabled
}

/// Why the server switched a webhook off. Raw values are the wire values.
public enum WebhookDisabledReason: String, Sendable {
  case failures
  case gone
}

/// Why a delivery failed. Raw values are the wire values.
public enum WebhookFailureReason: String, CaseIterable, Sendable {
  case gone
  case httpStatus = "http_status"
  case blockedAddress = "blocked_address"
  case timeout
  case tls
  case connection
}

/// The outcome of saving a webhook.
public struct WebhookSaveResult: Equatable, Sendable {
  public let webhook: Webhook
  /// The signing secret. Present only when the save created the webhook, and
  /// never returned again.
  public let secret: String?

  public init(webhook: Webhook, secret: String?) {
    self.webhook = webhook
    self.secret = secret
  }
}

/// The outcome of a test event.
public struct WebhookTestResult: Equatable, Sendable {
  public let delivered: Bool
  public let status: Int?
  public let failureReason: WebhookFailureReason?

  public init(delivered: Bool, status: Int?, failureReason: WebhookFailureReason?) {
    self.delivered = delivered
    self.status = status
    self.failureReason = failureReason
  }
}
