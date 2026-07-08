import TownCrierDomain

extension AppCoordinator {
  // MARK: - Watch Zone Factories

  public func makeWatchZoneListViewModel() -> WatchZoneListViewModel {
    // Reuse the cached VM only while its gate still reflects the current tier.
    // The tier resolves from the default `.free` to a paid tier *after* the VM
    // is first built (resolveSubscriptionTier runs asynchronously at launch),
    // and the list is keyed `.id(subscriptionTier)` so the view rebuilds on the
    // change. Returning a stale `.free` gate kept the upgrade badge on the +
    // button once the 1-zone free limit was hit, blocking paid users below
    // their actual limit (tc-ujct). Rebuild with a fresh gate when the tier
    // differs; same-tier calls still return a stable instance so the editor's
    // post-save reload path converges on the retained VM.
    if let cached = watchZoneListViewModel, cached.featureGate.tier == subscriptionTier {
      return cached
    }
    let viewModel = WatchZoneListViewModel(
      repository: watchZoneRepository,
      featureGate: FeatureGate(tier: subscriptionTier),
      deviceLocalZoneRepository: deviceLocalZoneRepository
    )
    viewModel.onAddZone = { [weak self] in
      self?.isAddingWatchZone = true
    }
    viewModel.onEditZone = { [weak self] zone in
      self?.editingWatchZone = zone
    }
    viewModel.onViewPlans = { [weak self] in
      self?.isSubscriptionPresented = true
    }
    // Unconverted device-local zones row (GH#879 Phase 5): reopens the same
    // conversion sheet completeOnboarding() presents post-wizard.
    viewModel.onConvertLocalZones = { [weak self] in
      self?.reopenDeviceLocalZoneConversion()
    }
    watchZoneListViewModel = viewModel
    return viewModel
  }

  public func makeWatchZoneEditorViewModel(
    editing zone: WatchZone? = nil
  ) -> WatchZoneEditorViewModel {
    guard let geocoder else {
      fatalError("PostcodeGeocoder must be injected to create WatchZoneEditorViewModel")
    }
    let viewModel = WatchZoneEditorViewModel(
      geocoder: geocoder,
      repository: watchZoneRepository,
      tier: subscriptionTier,
      editing: zone
    )
    let isEditing = zone != nil
    viewModel.onUpgradeRequired = { [weak self] in
      // Quota breach on save (tc-gpjk): close the editor sheet and present the
      // subscription paywall instead of showing an inline error.
      self?.isAddingWatchZone = false
      self?.editingWatchZone = nil
      self?.isSubscriptionPresented = true
      // Hitting a quota wall is a friction moment — never ask for a review in
      // the same session (GH #628).
      self?.reviewPromptTracker?.suppressThisSession()
    }
    viewModel.onSave = { [weak self] saved in
      self?.isAddingWatchZone = false
      self?.editingWatchZone = nil
      self?.pendingWatchZoneRefresh = Task { [weak self] in
        // Invalidate the per-zone applications cache before reloading so a
        // radius/centre change does not serve a stale cache hit on the
        // Apps view for up to the cache TTL (tc-9vid).
        if isEditing, let offlineRepository = self?.offlineRepository {
          await offlineRepository.invalidateCache(for: saved.id)
        }
        await self?.watchZoneListViewModel?.load()
      }
    }
    return viewModel
  }

  /// Test-only synchronisation: await the most recently kicked-off post-save
  /// watch-zone reload so assertions happen after the list view-model has
  /// been refreshed against the latest repository state. Replaces flaky
  /// `Task.sleep(...)` waits in tests.
  public func waitForPendingWatchZoneRefresh() async {
    await pendingWatchZoneRefresh?.value
  }
}
