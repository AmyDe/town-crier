import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Cache-invalidation behaviour for the watch-zone editor's onSave path.
/// Covers tc-9vid: editing a zone's radius/centre must invalidate the
/// per-zone applications cache so the Apps view does not serve a
/// stale-radius hit for up to the cache TTL.
@Suite("AppCoordinator -- Cache Invalidation On Zone Edit")
@MainActor
struct AppCoordinatorCacheInvalidationTests {
  private func makeOfflineRepository(
    cache: SpyApplicationCacheStore
  ) -> OfflineAwareRepository {
    OfflineAwareRepository(
      remote: SpyPlanningApplicationRepository(),
      cache: cache,
      connectivity: StubConnectivityMonitor(isConnected: true)
    )
  }

  private func makeSUT(
    watchZoneSpy: SpyWatchZoneRepository,
    offlineRepository: OfflineAwareRepository?
  ) -> AppCoordinator {
    AppCoordinator(
      repository: SpyPlanningApplicationRepository(),
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      offlineRepository: offlineRepository,
      watchZoneRepository: watchZoneSpy,
      geocoder: SpyPostcodeGeocoder(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService()
    )
  }

  /// Editing a zone (radius/centre change) must invalidate the per-zone
  /// applications cache so the Apps view does not serve a stale-radius
  /// cache hit for up to the cache TTL after the edit.
  @Test func editorOnSave_forEdit_invalidatesOfflineCacheForZone() async throws {
    let watchZoneSpy = SpyWatchZoneRepository()
    watchZoneSpy.loadAllResult = .success([.cambridge])
    let cacheSpy = SpyApplicationCacheStore()
    let offlineRepository = makeOfflineRepository(cache: cacheSpy)
    let sut = makeSUT(watchZoneSpy: watchZoneSpy, offlineRepository: offlineRepository)

    let resized = try WatchZone(
      id: WatchZone.cambridge.id,
      name: WatchZone.cambridge.name,
      centre: WatchZone.cambridge.centre,
      radiusMetres: 200,
      authorityId: WatchZone.cambridge.authorityId
    )
    sut.editingWatchZone = .cambridge
    let editorVM = sut.makeWatchZoneEditorViewModel(editing: .cambridge)
    editorVM.onSave?(resized)

    await sut.waitForPendingWatchZoneRefresh()

    #expect(cacheSpy.invalidateCalls == [WatchZone.cambridge.id])
  }

  /// Adding a new zone has no prior cache entry, so the editor's onSave
  /// path must not call invalidate — keeps the wire minimal and the spy
  /// traffic truthful.
  @Test func editorOnSave_forAdd_doesNotInvalidateOfflineCache() async throws {
    let watchZoneSpy = SpyWatchZoneRepository()
    watchZoneSpy.loadAllResult = .success([.cambridge])
    let cacheSpy = SpyApplicationCacheStore()
    let offlineRepository = makeOfflineRepository(cache: cacheSpy)
    let sut = makeSUT(watchZoneSpy: watchZoneSpy, offlineRepository: offlineRepository)

    sut.isAddingWatchZone = true
    let editorVM = sut.makeWatchZoneEditorViewModel()
    editorVM.onSave?(.cambridge)

    await sut.waitForPendingWatchZoneRefresh()

    #expect(cacheSpy.invalidateCalls.isEmpty)
  }

  /// When no offline repository is wired, the coordinator must still
  /// dismiss the editor and refresh the zones list without crashing — the
  /// invalidation path is best-effort.
  @Test func editorOnSave_forEdit_withoutOfflineRepository_doesNotCrash() async throws {
    let watchZoneSpy = SpyWatchZoneRepository()
    watchZoneSpy.loadAllResult = .success([.cambridge])
    let sut = makeSUT(watchZoneSpy: watchZoneSpy, offlineRepository: nil)

    sut.editingWatchZone = .cambridge
    let editorVM = sut.makeWatchZoneEditorViewModel(editing: .cambridge)
    editorVM.onSave?(.cambridge)

    await sut.waitForPendingWatchZoneRefresh()

    #expect(sut.editingWatchZone == nil)
  }
}
