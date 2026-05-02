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

  /// Loads the user's authority list for the Search tab. Returns an empty list
  /// on failure or when no authority repository is wired — Search will still
  /// render with a disabled picker.
  public func loadSearchAuthorities() async -> [LocalAuthority] {
    guard let authorityRepository else { return [] }
    do {
      let result = try await authorityRepository.fetchAuthorities()
      return result.authorities
    } catch {
      return []
    }
  }
}
