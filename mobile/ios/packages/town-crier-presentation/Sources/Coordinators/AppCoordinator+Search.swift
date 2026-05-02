import TownCrierDomain

extension AppCoordinator {

  /// Factory for the dedicated Search tab view model.
  ///
  /// Falls back to a no-op repository when no ``SearchRepository`` was
  /// injected (e.g. in tests that don't exercise search). Free/personal-tier
  /// users still receive a working ViewModel — the soft-paywall flow is
  /// driven by the ``FeatureGate`` derived from the current
  /// ``subscriptionTier``.
  public func makeSearchViewModel() -> SearchViewModel {
    let repository = searchRepository ?? UnavailableSearchRepository()
    let viewModel = SearchViewModel(
      repository: repository,
      featureGate: FeatureGate(tier: subscriptionTier)
    )
    viewModel.onApplicationSelected = { [weak self] id in
      self?.showApplicationDetail(id)
    }
    return viewModel
  }
}
