import Foundation
import Testing

@testable import TownCrierDomain
@testable import TownCrierPresentation

/// Pre-canned filter selection, the Pro-tier entitlement gate, and the
/// fetched-catalog display-name resolution on `WatchZoneEditorViewModel`
/// (GH#1098/tc-hogkn, GH#1104/tc-m8j90.2). Mirrors
/// `WatchZoneEditorViewModelShapeModeTests`'s structure -- the closest
/// existing precedent (GH#1031's Circle/Custom shape gate), not the
/// `GatedToggle`/`Entitlement` sheet mechanism. `selectedFilterKey` is a
/// plain opaque `String?` (GH#1104): it is never validated against the
/// fetched catalog, only its *display* is.
@MainActor
@Suite("WatchZoneEditorViewModel — pre-canned filter")
struct WatchZoneEditorViewModelFilterTests {
  private var spyGeocoder: SpyPostcodeGeocoder!
  private var spyRepository: SpyWatchZoneRepository!

  init() {
    spyGeocoder = SpyPostcodeGeocoder()
    spyRepository = SpyWatchZoneRepository()
  }

  private func makeSUT(
    tier: SubscriptionTier = .pro,
    editing zone: WatchZone? = nil
  ) -> WatchZoneEditorViewModel {
    WatchZoneEditorViewModel(
      geocoder: spyGeocoder,
      repository: spyRepository,
      tier: tier,
      editing: zone
    )
  }

  private func filteredZone(filterKey: String = "loft_extension") throws -> WatchZone {
    try WatchZone(
      id: WatchZoneId("zone-filtered"),
      name: "Filtered Area",
      centre: .cambridge,
      radiusMetres: 1500,
      filterKey: filterKey
    )
  }

  // MARK: - Initial state

  @Test func initialState_selectedFilterKeyIsNil() {
    let sut = makeSUT()
    #expect(sut.selectedFilterKey == nil)
  }

  @Test func initialState_editingFilteredZone_populatesSelectedFilterKey() throws {
    let sut = makeSUT(editing: try filteredZone())

    #expect(sut.selectedFilterKey == "loft_extension")
  }

  @Test func initialState_editingUnfilteredZone_selectedFilterKeyStaysNil() {
    let sut = makeSUT(editing: .cambridge)

    #expect(sut.selectedFilterKey == nil)
  }

  @Test func initialState_filterCatalogIsEmptyUntilFetched() {
    let sut = makeSUT()

    #expect(sut.filterCatalog.isEmpty)
    #expect(!sut.isLoadingFilterCatalog)
    #expect(!sut.filterCatalogLoadFailed)
    #expect(spyRepository.filterCatalogCallCount == 0)
  }

  // MARK: - canSetWatchZoneFilter (Pro-only tier gating)

  @Test func canSetWatchZoneFilter_falseOnFreeTier() {
    #expect(!makeSUT(tier: .free).canSetWatchZoneFilter)
  }

  @Test func canSetWatchZoneFilter_falseOnPersonalTier() {
    #expect(!makeSUT(tier: .personal).canSetWatchZoneFilter)
  }

  @Test func canSetWatchZoneFilter_trueOnProTier() {
    #expect(makeSUT(tier: .pro).canSetWatchZoneFilter)
  }

  // MARK: - isFilterLocked (downgraded Pro tier holding a filtered zone)

  @Test func isFilterLocked_falseByDefault() {
    #expect(!makeSUT().isFilterLocked)
  }

  @Test func isFilterLocked_falseOnProTier_editingFilteredZone() throws {
    let sut = makeSUT(tier: .pro, editing: try filteredZone())

    #expect(!sut.isFilterLocked)
  }

  @Test func isFilterLocked_trueOnFreeTier_editingFilteredZone() throws {
    // A downgraded Pro user reopening the editor for a filtered zone they
    // set before downgrading -- same posture as isCustomShapeLocked.
    let sut = makeSUT(tier: .free, editing: try filteredZone())

    #expect(sut.selectedFilterKey == "loft_extension")
    #expect(sut.isFilterLocked)
  }

  @Test func isFilterLocked_trueOnPersonalTier_editingFilteredZone() throws {
    let sut = makeSUT(tier: .personal, editing: try filteredZone())

    #expect(sut.isFilterLocked)
  }

  @Test func isFilterLocked_falseOnFreeTier_editingUnfilteredZone() {
    let sut = makeSUT(tier: .free, editing: .cambridge)

    #expect(!sut.isFilterLocked)
  }

  // MARK: - selectFilterKey

  @Test func selectFilterKey_settingOnProTier_isAllowed() {
    let sut = makeSUT(tier: .pro)

    sut.selectFilterKey("hmo_house_shares")

    #expect(sut.selectedFilterKey == "hmo_house_shares")
  }

  @Test func selectFilterKey_everyCatalogEntry_settableOnProTier() {
    let sut = makeSUT(tier: .pro)

    for entry in FilterCatalogEntry.allCatalog {
      sut.selectFilterKey(entry.key)
      #expect(sut.selectedFilterKey == entry.key)
    }
  }

  @Test func selectFilterKey_settingOnFreeTier_isSilentNoOp() {
    let sut = makeSUT(tier: .free)

    sut.selectFilterKey("hmo_house_shares")

    #expect(sut.selectedFilterKey == nil)
  }

  @Test func selectFilterKey_settingOnPersonalTier_isSilentNoOp() {
    let sut = makeSUT(tier: .personal)

    sut.selectFilterKey("hmo_house_shares")

    #expect(sut.selectedFilterKey == nil)
  }

  @Test func selectFilterKey_clearingToNil_alwaysAllowedRegardlessOfTier() throws {
    // Clearing never requires an entitlement check, mirroring the server's
    // tri-state PATCH semantics (GH#1090): a downgraded Free-tier user must
    // still be able to clear a filter they can no longer set.
    let sut = makeSUT(tier: .free, editing: try filteredZone())
    #expect(sut.selectedFilterKey == "loft_extension")

    sut.selectFilterKey(nil)

    #expect(sut.selectedFilterKey == nil)
  }

  // MARK: - loadFilterCatalogIfNeeded (GH#1104, tc-m8j90.2)

  @Test func loadFilterCatalogIfNeeded_success_populatesFilterCatalog() async {
    let sut = makeSUT()
    spyRepository.filterCatalogResult = .success(FilterCatalogEntry.allCatalog)

    await sut.loadFilterCatalogIfNeeded()

    #expect(sut.filterCatalog == FilterCatalogEntry.allCatalog)
    #expect(!sut.isLoadingFilterCatalog)
    #expect(!sut.filterCatalogLoadFailed)
  }

  @Test func loadFilterCatalogIfNeeded_calledTwice_fetchesOnlyOnce() async {
    // Memoized for the editor session -- re-opening the picker within the
    // same session must not re-fetch (GH#1104's "once per app session"
    // decision).
    let sut = makeSUT()
    spyRepository.filterCatalogResult = .success(FilterCatalogEntry.allCatalog)

    await sut.loadFilterCatalogIfNeeded()
    await sut.loadFilterCatalogIfNeeded()

    #expect(spyRepository.filterCatalogCallCount == 1)
  }

  @Test func loadFilterCatalogIfNeeded_failure_setsFilterCatalogLoadFailed() async {
    let sut = makeSUT()
    spyRepository.filterCatalogResult = .failure(DomainError.networkUnavailable)

    await sut.loadFilterCatalogIfNeeded()

    #expect(sut.filterCatalog.isEmpty)
    #expect(!sut.isLoadingFilterCatalog)
    #expect(sut.filterCatalogLoadFailed)
  }

  @Test func loadFilterCatalogIfNeeded_failureThenRetrySucceeds_populatesFilterCatalog() async {
    // A failed fetch leaves hasLoadedFilterCatalog false, so calling this
    // method again (the picker's retry row) re-attempts rather than
    // silently staying stuck failed.
    let sut = makeSUT()
    spyRepository.filterCatalogResult = .failure(DomainError.networkUnavailable)
    await sut.loadFilterCatalogIfNeeded()
    #expect(sut.filterCatalogLoadFailed)

    spyRepository.filterCatalogResult = .success(FilterCatalogEntry.allCatalog)
    await sut.loadFilterCatalogIfNeeded()

    #expect(sut.filterCatalog == FilterCatalogEntry.allCatalog)
    #expect(!sut.filterCatalogLoadFailed)
    #expect(spyRepository.filterCatalogCallCount == 2)
  }

  @Test func loadFilterCatalogIfNeeded_failure_neverClearsSelectedFilterKey() async throws {
    // The direct fix for tc-8z9ri: a display-layer fetch failure must never
    // touch the stored filter.
    let sut = makeSUT(editing: try filteredZone())
    spyRepository.filterCatalogResult = .failure(DomainError.networkUnavailable)

    await sut.loadFilterCatalogIfNeeded()

    #expect(sut.selectedFilterKey == "loft_extension")
  }

  // MARK: - filterDisplayName(for:) (GH#1104, tc-m8j90.2 -- fixes tc-8z9ri)

  @Test func filterDisplayName_nilKey_returnsNil() {
    let sut = makeSUT()

    #expect(sut.filterDisplayName(for: nil) == nil)
  }

  @Test func filterDisplayName_keyInFetchedCatalog_returnsItsDisplayName() async {
    let sut = makeSUT()
    spyRepository.filterCatalogResult = .success(FilterCatalogEntry.allCatalog)
    await sut.loadFilterCatalogIfNeeded()

    #expect(sut.filterDisplayName(for: "loft_extension") == "Loft extension")
  }

  /// The key regression test for tc-8z9ri: a `selectedFilterKey` not present
  /// in the fetched catalog (fetch pending, fetch failed, or a genuinely
  /// newer key an older client build predates) must still resolve to a
  /// non-nil fallback label -- never "None" -- and the stored key itself
  /// must be untouched.
  @Test func filterDisplayName_keyNotInFetchedCatalog_returnsFallbackLabel_neverNil() async throws {
    let sut = makeSUT(editing: try filteredZone(filterKey: "a_filter_this_build_predates"))
    spyRepository.filterCatalogResult = .success(FilterCatalogEntry.allCatalog)
    await sut.loadFilterCatalogIfNeeded()

    let displayName = sut.filterDisplayName(for: sut.selectedFilterKey)

    #expect(displayName != nil)
    #expect(displayName != "None")
    #expect(sut.selectedFilterKey == "a_filter_this_build_predates")
  }

  @Test func filterDisplayName_catalogNeverFetched_returnsFallbackLabel_neverNil() throws {
    // Fetch never even attempted (e.g. the View hasn't shown the picker
    // yet) -- filterCatalog is empty, but a set filter must still read as
    // set.
    let sut = makeSUT(editing: try filteredZone())

    let displayName = sut.filterDisplayName(for: sut.selectedFilterKey)

    #expect(displayName != nil)
    #expect(displayName != "None")
  }

  // MARK: - save() sends the selected filter key

  private func geocode(_ sut: WatchZoneEditorViewModel) async {
    sut.postcodeInput = "CB1 2AD"
    spyGeocoder.geocodeResult = .success(.cambridge)
    await sut.submitPostcode()
  }

  @Test func save_proTier_sendsSelectedFilterKey() async {
    let sut = makeSUT(tier: .pro)
    await geocode(sut)
    sut.selectFilterKey("new_homes")

    let didSave = await sut.save()

    #expect(didSave)
    let saved = spyRepository.saveCalls.first
    #expect(saved?.filterKey == "new_homes")
  }

  @Test func save_noFilterSelected_sendsNilFilterKey() async {
    let sut = makeSUT(tier: .pro)
    await geocode(sut)

    let didSave = await sut.save()

    #expect(didSave)
    let saved = spyRepository.saveCalls.first
    #expect(saved?.filterKey == nil)
  }

  @Test func save_editingLockedFilteredZone_stillSendsExistingFilterKey() async throws {
    // A downgraded tier can still save the zone (renaming, toggling
    // notifications) without losing the locked filter -- it's read-only, not
    // silently cleared.
    let sut = makeSUT(tier: .free, editing: try filteredZone())

    let didSave = await sut.save()

    #expect(didSave)
    let saved = spyRepository.updateCalls.first
    #expect(saved?.filterKey == "loft_extension")
  }
}
