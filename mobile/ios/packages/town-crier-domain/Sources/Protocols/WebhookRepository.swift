/// Manages the signed-in user's alert webhook.
public protocol WebhookRepository: Sendable {
  /// Returns the user's webhook, or `nil` when none is set.
  func fetch() async throws -> Webhook?

  /// Creates the webhook, or replaces its URL and format and switches it on
  /// again. Throws `DomainError.webhookURLInvalid`, `.webhookFormatInvalid`
  /// or `.webhookRequiresPro` when the server rejects the request.
  func save(url: String, format: WebhookFormat) async throws -> WebhookSaveResult

  /// Deletes the webhook. Succeeds when none exists.
  func remove() async throws

  /// Replaces the signing secret and returns the new one. Throws
  /// `DomainError.webhookNotFound` or `.webhookRequiresPro`.
  func rotateSecret() async throws -> String

  /// Sends a test event and waits for the outcome. Throws
  /// `DomainError.webhookNotFound`, `.webhookRequiresPro` or
  /// `.webhookTestCooldown`.
  func sendTest() async throws -> WebhookTestResult
}
