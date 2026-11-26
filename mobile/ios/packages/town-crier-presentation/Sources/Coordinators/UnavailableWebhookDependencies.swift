import TownCrierDomain

/// Stands in when no webhook repository is injected: every call fails.
struct UnavailableWebhookRepository: WebhookRepository {
  private var failure: DomainError { .unexpected("Webhooks are unavailable") }

  func fetch() async throws -> Webhook? { throw failure }

  func save(url: String, format: WebhookFormat) async throws -> WebhookSaveResult { throw failure }

  func remove() async throws { throw failure }

  func rotateSecret() async throws -> String { throw failure }

  func sendTest() async throws -> WebhookTestResult { throw failure }
}

/// Stands in when no clipboard is injected: copying does nothing.
struct UnavailableClipboardWriter: ClipboardWriter {
  func copy(_ text: String) {}
}
