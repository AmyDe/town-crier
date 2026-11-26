import TownCrierDomain

final class SpyWebhookRepository: WebhookRepository, @unchecked Sendable {
  private(set) var fetchCallCount = 0
  var fetchResult: Result<Webhook?, Error> = .success(nil)

  func fetch() async throws -> Webhook? {
    fetchCallCount += 1
    return try fetchResult.get()
  }

  private(set) var saveCalls: [(url: String, format: WebhookFormat)] = []
  var saveResult: Result<WebhookSaveResult, Error> = .success(
    WebhookSaveResult(webhook: .fixture(), secret: nil))
  /// Runs while `save` is in flight.
  var whileSaving: (@Sendable () async -> Void)?

  func save(url: String, format: WebhookFormat) async throws -> WebhookSaveResult {
    saveCalls.append((url, format))
    await whileSaving?()
    return try saveResult.get()
  }

  private(set) var removeCallCount = 0
  var removeResult: Result<Void, Error> = .success(())

  func remove() async throws {
    removeCallCount += 1
    try removeResult.get()
  }

  private(set) var rotateSecretCallCount = 0
  var rotateSecretResult: Result<String, Error> = .success("whsec_rotated")

  func rotateSecret() async throws -> String {
    rotateSecretCallCount += 1
    return try rotateSecretResult.get()
  }

  private(set) var sendTestCallCount = 0
  var sendTestResult: Result<WebhookTestResult, Error> = .success(
    WebhookTestResult(delivered: true, status: 200, failureReason: nil))

  func sendTest() async throws -> WebhookTestResult {
    sendTestCallCount += 1
    return try sendTestResult.get()
  }
}
