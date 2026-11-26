import TownCrierDomain

extension AppCoordinator {
  /// Pushes the Webhooks screen from Settings.
  public func showWebhooks() {
    isWebhooksPresented = true
  }

  /// Factory for the Webhooks ViewModel, built for the current subscription tier.
  public func makeWebhookViewModel() -> WebhookViewModel {
    WebhookViewModel(
      repository: webhookRepository,
      clipboard: clipboardWriter,
      tier: subscriptionTier
    ) { [weak self] in
      self?.showPlansFromSettings()
    }
  }
}
