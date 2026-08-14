import Foundation
import Testing

@testable import TownCrierDomain
@testable import TownCrierPresentation

/// Pre-canned filter selection and the Pro-tier entitlement gate on
/// `WatchZoneEditorViewModel` (GH#1098, bead tc-hogkn). Mirrors
/// `WatchZoneEditorViewModelShapeModeTests`'s structure -- the closest
/// existing precedent (GH#1031's Circle/Custom shape gate), not the
/// `GatedToggle`/`Entitlement` sheet mechanism.
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

  private func filteredZone(filterKey: WatchZoneFilterKey = .loftExtension) throws -> WatchZone {
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

    #expect(sut.selectedFilterKey == .loftExtension)
  }

  @Test func initialState_editingUnfilteredZone_selectedFilterKeyStaysNil() {
    let sut = makeSUT(editing: .cambridge)

    #expect(sut.selectedFilterKey == nil)
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

    #expect(sut.selectedFilterKey == .loftExtension)
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

    sut.selectFilterKey(.hmoHouseShares)

    #expect(sut.selectedFilterKey == .hmoHouseShares)
  }

  @Test func selectFilterKey_everyCatalogEntry_settableOnProTier() {
    let sut = makeSUT(tier: .pro)

    for filterKey in WatchZoneFilterKey.allCases {
      sut.selectFilterKey(filterKey)
      #expect(sut.selectedFilterKey == filterKey)
    }
  }

  @Test func selectFilterKey_settingOnFreeTier_isSilentNoOp() {
    let sut = makeSUT(tier: .free)

    sut.selectFilterKey(.hmoHouseShares)

    #expect(sut.selectedFilterKey == nil)
  }

  @Test func selectFilterKey_settingOnPersonalTier_isSilentNoOp() {
    let sut = makeSUT(tier: .personal)

    sut.selectFilterKey(.hmoHouseShares)

    #expect(sut.selectedFilterKey == nil)
  }

  @Test func selectFilterKey_clearingToNil_alwaysAllowedRegardlessOfTier() throws {
    // Clearing never requires an entitlement check, mirroring the server's
    // tri-state PATCH semantics (GH#1090): a downgraded Free-tier user must
    // still be able to clear a filter they can no longer set.
    let sut = makeSUT(tier: .free, editing: try filteredZone())
    #expect(sut.selectedFilterKey == .loftExtension)

    sut.selectFilterKey(nil)

    #expect(sut.selectedFilterKey == nil)
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
    sut.selectFilterKey(.newHomes)

    let didSave = await sut.save()

    #expect(didSave)
    let saved = spyRepository.saveCalls.first
    #expect(saved?.filterKey == .newHomes)
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
    #expect(saved?.filterKey == .loftExtension)
  }
}
