import TownCrierDomain

/// Post-signup conversion of leftover device-local zones (GH#879 Phase 5).
/// The wizard's own zone (if any) is converted separately, via
/// `AppCoordinator+Onboarding`'s prefill/`completeOnboarding()` seam — this
/// extension handles everything left over after that: a one-time sheet
/// offered immediately post-wizard, reachable again afterwards from the
/// authed Zones tab's dismissible row for as long as any zones remain.
extension AppCoordinator {
  /// Presents the conversion sheet if any device-local zones remain after
  /// the wizard's own conversion. Called from `completeOnboarding()`; a
  /// no-op when no repository was injected or nothing is left to offer.
  func presentDeviceLocalZoneConversionIfNeeded() {
    guard let deviceLocalZoneRepository, !deviceLocalZoneRepository.loadAll().isEmpty else {
      return
    }
    isDeviceLocalZoneConversionPresented = true
  }

  /// Builds the conversion sheet's view model from the current local-zone
  /// state. Rebuilt fresh on every presentation (post-wizard, or reopened
  /// from the Zones tab row) so it never shows already-converted or
  /// already-deleted zones. Returns `nil` when no repository was injected —
  /// the sheet's content closure degrades to showing nothing rather than
  /// crashing.
  public func makeDeviceLocalZoneConversionViewModel() -> DeviceLocalZoneConversionViewModel? {
    guard let deviceLocalZoneRepository else { return nil }
    let viewModel = DeviceLocalZoneConversionViewModel(
      zones: deviceLocalZoneRepository.loadAll(),
      watchZoneRepository: watchZoneRepository,
      deviceLocalZoneRepository: deviceLocalZoneRepository
    )
    viewModel.onInsufficientEntitlement = { [weak self] in
      // Quota breach mid-conversion (tc-gpjk precedent): close this sheet and
      // present the subscription paywall instead of showing an inline error.
      self?.isDeviceLocalZoneConversionPresented = false
      self?.isSubscriptionPresented = true
      self?.refreshAfterDeviceLocalZoneConversion()
    }
    viewModel.onFinished = { [weak self] in
      self?.isDeviceLocalZoneConversionPresented = false
      self?.refreshAfterDeviceLocalZoneConversion()
    }
    return viewModel
  }

  /// Reopens the conversion sheet from the authed Zones tab's dismissible
  /// row (``WatchZoneListViewModel/onConvertLocalZones``). The sheet's
  /// content is rebuilt fresh via `makeDeviceLocalZoneConversionViewModel()`
  /// at presentation time, so this only needs to flip the flag.
  public func reopenDeviceLocalZoneConversion() {
    isDeviceLocalZoneConversionPresented = true
  }

  /// Refreshes the cached authed Zones list view model after a conversion
  /// pass, so newly-created zones (and the now-shorter/empty unconverted-zone
  /// row) show up without waiting for the next tab appearance.
  private func refreshAfterDeviceLocalZoneConversion() {
    pendingDeviceLocalZoneConversionRefresh = Task { [weak self] in
      await self?.watchZoneListViewModel?.load()
    }
  }

  /// Test-only synchronisation: await the most recently kicked-off post-
  /// conversion refresh, mirroring `waitForPendingWatchZoneRefresh()`.
  public func waitForPendingDeviceLocalZoneConversionRefresh() async {
    await pendingDeviceLocalZoneConversionRefresh?.value
  }
}
