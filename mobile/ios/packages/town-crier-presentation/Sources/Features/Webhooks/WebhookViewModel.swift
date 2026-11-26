import Combine
import Foundation
import TownCrierDomain

/// Drives the Webhooks screen: loading, saving, testing, rotating and removing
/// the user's alert webhook.
@MainActor
public final class WebhookViewModel: ObservableObject, ErrorHandlingViewModel {
  public enum State: Equatable, Sendable {
    case loading
    case loadFailed
    case upsell
    case empty
    case configured(Webhook)
  }

  @Published public private(set) var state: State = .loading
  @Published public var urlText = ""
  @Published public var format: WebhookFormat = .json
  // The server returns a secret only once, so it lives here and nowhere else.
  @Published public private(set) var revealedSecret: String?
  @Published public private(set) var testResult: WebhookTestResult?
  @Published public private(set) var inlineError: String?
  @Published public private(set) var isBusy = false
  @Published public var isRemoveAlertPresented = false
  @Published var error: DomainError?

  let timeZone: TimeZone

  private let repository: WebhookRepository
  private let clipboard: ClipboardWriter
  private let tier: SubscriptionTier
  private let onViewPlans: () -> Void

  public init(
    repository: WebhookRepository,
    clipboard: ClipboardWriter,
    tier: SubscriptionTier,
    timeZone: TimeZone = .current,
    onViewPlans: @escaping () -> Void
  ) {
    self.repository = repository
    self.clipboard = clipboard
    self.tier = tier
    self.timeZone = timeZone
    self.onViewPlans = onViewPlans
  }

  public var canSave: Bool {
    !isBusy && !trimmedURL.isEmpty
  }

  private var trimmedURL: String {
    urlText.trimmingCharacters(in: .whitespacesAndNewlines)
  }

  public func load() async {
    state = .loading
    inlineError = nil
    error = nil
    do {
      if let webhook = try await repository.fetch() {
        show(webhook)
      } else {
        showNoWebhook()
      }
    } catch {
      guard !error.isCancellation else { return }
      state = .loadFailed
      if isSharedError(error) {
        handleError(error)
      }
    }
  }

  public func save() async {
    guard canSave else { return }
    await perform(failureCopy: WebhookCopy.errorSave) {
      let result = try await repository.save(url: trimmedURL, format: format)
      show(result.webhook)
      revealedSecret = result.secret
    }
  }

  public func sendTest() async {
    await perform(failureCopy: WebhookCopy.errorTest) {
      testResult = try await repository.sendTest()
    }
  }

  public func rotateSecret() async {
    await perform(failureCopy: WebhookCopy.errorRotate) {
      revealedSecret = try await repository.rotateSecret()
    }
  }

  public func requestRemove() {
    isRemoveAlertPresented = true
  }

  public func confirmRemove() async {
    isRemoveAlertPresented = false
    await perform(failureCopy: WebhookCopy.errorRemove) {
      try await repository.remove()
      showNoWebhook()
    }
  }

  public func copySecret() {
    guard let revealedSecret else { return }
    clipboard.copy(revealedSecret)
  }

  public func viewPlans() {
    onViewPlans()
  }

  /// Forgets everything that must not outlive the screen.
  public func reset() {
    revealedSecret = nil
    testResult = nil
  }

  private func perform(failureCopy: String, _ operation: () async throws -> Void) async {
    guard !isBusy else { return }
    isBusy = true
    defer { isBusy = false }
    inlineError = nil
    error = nil
    testResult = nil
    do {
      try await operation()
    } catch {
      report(error, failureCopy: failureCopy)
    }
  }

  private func report(_ failure: Error, failureCopy: String) {
    guard !failure.isCancellation else { return }
    if isSharedError(failure) {
      handleError(failure)
      return
    }
    switch failure as? DomainError {
    case .webhookURLInvalid:
      inlineError = WebhookCopy.errorURLInvalid
    case .webhookFormatInvalid:
      inlineError = WebhookCopy.errorFormatInvalid
    case .webhookRequiresPro:
      inlineError = WebhookCopy.errorRequiresPro
    case .webhookTestCooldown:
      inlineError = WebhookCopy.errorCooldown
    case .webhookNotFound:
      showNoWebhook()
      inlineError = WebhookCopy.errorNotFound
    default:
      inlineError = failureCopy
    }
  }

  private func isSharedError(_ failure: Error) -> Bool {
    switch failure as? DomainError {
    case .sessionExpired, .networkUnavailable: true
    default: false
    }
  }

  private func show(_ webhook: Webhook) {
    state = .configured(webhook)
    urlText = webhook.url
    format = webhook.format
  }

  private func showNoWebhook() {
    state = tier == .pro ? .empty : .upsell
    urlText = ""
    format = .json
    reset()
  }
}
