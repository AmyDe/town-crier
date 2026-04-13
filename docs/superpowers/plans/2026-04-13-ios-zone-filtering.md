# iOS Zone Filtering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a horizontal zone pill bar to the Applications and Map tabs so users with multiple watch zones can switch between them.

**Architecture:** Reusable `ZonePickerView` component integrated into both tabs. Each tab's ViewModel independently manages zone loading, selection, and UserDefaults persistence. No coordinator or shared state changes needed — ViewModels gain `userDefaults` and `zoneSelectionKey` init parameters with defaults, so existing call sites are unaffected.

**Tech Stack:** Swift, SwiftUI, Apple Testing framework, manual spies

**Spec:** `docs/specs/ios-zone-filtering.md`

---

### Task 1: Create ZonePickerView component

**Files:**
- Create: `mobile/ios/packages/town-crier-presentation/Sources/Features/ZonePicker/ZonePickerView.swift`

- [ ] **Step 1: Create ZonePickerView**

```swift
import SwiftUI
import TownCrierDomain

/// Horizontal scrollable pill bar for switching between watch zones.
public struct ZonePickerView: View {
  let zones: [WatchZone]
  let selectedZoneId: WatchZoneId?
  let onSelect: (WatchZone) -> Void

  public init(
    zones: [WatchZone],
    selectedZoneId: WatchZoneId?,
    onSelect: @escaping (WatchZone) -> Void
  ) {
    self.zones = zones
    self.selectedZoneId = selectedZoneId
    self.onSelect = onSelect
  }

  public var body: some View {
    ScrollView(.horizontal, showsIndicators: false) {
      HStack(spacing: TCSpacing.small) {
        ForEach(zones) { zone in
          zoneChip(zone: zone, isSelected: zone.id == selectedZoneId)
        }
      }
      .padding(.horizontal, TCSpacing.medium)
      .padding(.vertical, TCSpacing.small)
    }
  }

  private func zoneChip(zone: WatchZone, isSelected: Bool) -> some View {
    Button {
      onSelect(zone)
    } label: {
      Text(zone.name)
        .font(TCTypography.captionEmphasis)
        .foregroundStyle(isSelected ? Color.tcTextOnAccent : Color.tcTextPrimary)
        .padding(.horizontal, TCSpacing.small)
        .padding(.vertical, TCSpacing.extraSmall)
        .background(isSelected ? Color.tcAmber : Color.tcSurface)
        .clipShape(Capsule())
        .overlay(
          Capsule()
            .stroke(Color.tcBorder, lineWidth: isSelected ? 0 : 1)
        )
    }
    .buttonStyle(.plain)
  }
}
```

The styling matches the existing `filterChip` in `ApplicationListView.swift:85-102` exactly — same colors, typography, spacing, and capsule shape.

- [ ] **Step 2: Verify build**

Run: `cd /Users/christy/Dev/town-crier/mobile/ios && swift build 2>&1 | tail -5`
Expected: Build succeeded

- [ ] **Step 3: Commit**

```bash
git add mobile/ios/packages/town-crier-presentation/Sources/Features/ZonePicker/ZonePickerView.swift
git commit -m "feat(ios): add ZonePickerView component"
```

---

### Task 2: ApplicationListViewModel — zone selection (TDD)

**Files:**
- Modify: `mobile/ios/packages/town-crier-presentation/Sources/Features/ApplicationList/ApplicationListViewModel.swift`
- Modify: `mobile/ios/town-crier-tests/Sources/Features/ApplicationListViewModelTests.swift`

- [ ] **Step 1: Write failing tests for zone selection**

Add a new `makeSUTWithZones` helper and zone selection tests to `ApplicationListViewModelTests.swift`. Add these after the existing `// MARK: - Empty State` section (after line 292):

```swift
  // MARK: - Zone Selection

  private func makeSUTWithZones(
    zones: [WatchZone] = [.cambridge, .london],
    applications: [PlanningApplication] = [.pendingReview],
    tier: SubscriptionTier = .free,
    persistedZoneId: String? = nil
  ) -> (ApplicationListViewModel, SpyPlanningApplicationRepository, SpyWatchZoneRepository, UserDefaults) {
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success(applications)
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success(zones)
    let defaults = UserDefaults(suiteName: UUID().uuidString)!
    if let persistedZoneId {
      defaults.set(persistedZoneId, forKey: "test.zone")
    }
    let vm = ApplicationListViewModel(
      watchZoneRepository: zoneSpy,
      repository: appSpy,
      tier: tier,
      userDefaults: defaults,
      zoneSelectionKey: "test.zone"
    )
    return (vm, appSpy, zoneSpy, defaults)
  }

  @Test func loadApplications_populatesZonesFromRepository() async {
    let (sut, _, _, _) = makeSUTWithZones()

    await sut.loadApplications()

    #expect(sut.zones.count == 2)
    #expect(sut.zones[0].id == WatchZone.cambridge.id)
    #expect(sut.zones[1].id == WatchZone.london.id)
  }

  @Test func loadApplications_selectsFirstZoneByDefault() async {
    let (sut, appSpy, _, _) = makeSUTWithZones()

    await sut.loadApplications()

    #expect(sut.selectedZone?.id == WatchZone.cambridge.id)
    #expect(appSpy.fetchApplicationsCalls.first?.id == WatchZone.cambridge.id)
  }

  @Test func loadApplications_restoresPersistedZoneSelection() async {
    let (sut, appSpy, _, _) = makeSUTWithZones(persistedZoneId: "zone-002")

    await sut.loadApplications()

    #expect(sut.selectedZone?.id == WatchZone.london.id)
    #expect(appSpy.fetchApplicationsCalls.first?.id == WatchZone.london.id)
  }

  @Test func loadApplications_fallsBackToFirstZone_whenPersistedZoneDeleted() async {
    let (sut, appSpy, _, _) = makeSUTWithZones(persistedZoneId: "zone-deleted")

    await sut.loadApplications()

    #expect(sut.selectedZone?.id == WatchZone.cambridge.id)
    #expect(appSpy.fetchApplicationsCalls.first?.id == WatchZone.cambridge.id)
  }

  @Test func selectZone_fetchesApplicationsForNewZone() async {
    let (sut, appSpy, _, _) = makeSUTWithZones()
    await sut.loadApplications()
    appSpy.fetchApplicationsCalls.removeAll()

    await sut.selectZone(.london)

    #expect(sut.selectedZone?.id == WatchZone.london.id)
    #expect(appSpy.fetchApplicationsCalls.count == 1)
    #expect(appSpy.fetchApplicationsCalls.first?.id == WatchZone.london.id)
  }

  @Test func selectZone_persistsSelectionToUserDefaults() async {
    let (sut, _, _, defaults) = makeSUTWithZones()
    await sut.loadApplications()

    await sut.selectZone(.london)

    #expect(defaults.string(forKey: "test.zone") == "zone-002")
  }

  @Test func selectZone_resetsStatusFilter() async {
    let (sut, _, _, _) = makeSUTWithZones(tier: .personal)
    await sut.loadApplications()
    sut.selectedStatusFilter = .approved

    await sut.selectZone(.london)

    #expect(sut.selectedStatusFilter == nil)
  }

  @Test func showZonePicker_trueWhenMultipleZones() async {
    let (sut, _, _, _) = makeSUTWithZones()
    await sut.loadApplications()

    #expect(sut.showZonePicker)
  }

  @Test func showZonePicker_falseWhenSingleZone() async {
    let (sut, _, _, _) = makeSUTWithZones(zones: [.cambridge])
    await sut.loadApplications()

    #expect(!sut.showZonePicker)
  }

  @Test func showZonePicker_falseWhenNoZones() async {
    let (sut, _, _, _) = makeSUTWithZones(zones: [])
    await sut.loadApplications()

    #expect(!sut.showZonePicker)
  }
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/christy/Dev/town-crier/mobile/ios && swift test 2>&1 | grep -E "FAIL|error:|Build|passed|failed"`
Expected: Compilation errors — `zones`, `selectedZone`, `showZonePicker`, `selectZone`, `userDefaults`, `zoneSelectionKey` don't exist yet.

- [ ] **Step 3: Implement ViewModel changes**

Modify `ApplicationListViewModel.swift`. Add new properties after the existing `@Published var error` (line 10):

```swift
  @Published private(set) var zones: [WatchZone] = []
  @Published private(set) var selectedZone: WatchZone?
```

Add stored properties after `private let tier` (line 16):

```swift
  private let userDefaults: UserDefaults
  private let zoneSelectionKey: String
```

Add computed property after `canFilter` (line 22):

```swift
  public var showZonePicker: Bool {
    zones.count > 1
  }
```

Update init 1 (`repository: PlanningApplicationRepository, zone: WatchZone`) — add after existing assignments:

```swift
    self.userDefaults = .standard
    self.zoneSelectionKey = ""
```

Update init 2 (`offlineRepository: OfflineAwareRepository, zone: WatchZone`) — same additions.

Update init 3 (`watchZoneRepository: WatchZoneRepository, repository: PlanningApplicationRepository`) — add parameters and assignments:

```swift
  public init(
    watchZoneRepository: WatchZoneRepository,
    repository: PlanningApplicationRepository,
    tier: SubscriptionTier = .free,
    userDefaults: UserDefaults = .standard,
    zoneSelectionKey: String = "lastSelectedZone.applications"
  ) {
    self.repository = repository
    self.offlineRepository = nil
    self.watchZoneRepository = watchZoneRepository
    self.zone = nil
    self.tier = tier
    self.userDefaults = userDefaults
    self.zoneSelectionKey = zoneSelectionKey
  }
```

Update init 4 (`watchZoneRepository: WatchZoneRepository, offlineRepository: OfflineAwareRepository`) — same pattern:

```swift
  public init(
    watchZoneRepository: WatchZoneRepository,
    offlineRepository: OfflineAwareRepository,
    tier: SubscriptionTier = .free,
    userDefaults: UserDefaults = .standard,
    zoneSelectionKey: String = "lastSelectedZone.applications"
  ) {
    self.repository = nil
    self.offlineRepository = offlineRepository
    self.watchZoneRepository = watchZoneRepository
    self.zone = nil
    self.tier = tier
    self.userDefaults = userDefaults
    self.zoneSelectionKey = zoneSelectionKey
  }
```

Replace the existing `loadApplications()` method (lines 96-125) with:

```swift
  public func loadApplications() async {
    isLoading = true
    error = nil
    do {
      if let watchZoneRepository {
        let loadedZones = try await watchZoneRepository.loadAll()
        zones = loadedZones
        if selectedZone == nil || !loadedZones.contains(where: { $0.id == selectedZone?.id }) {
          selectedZone = resolveInitialZone(from: loadedZones)
        }
      }
      guard let activeZone = selectedZone ?? zone else {
        applications = []
        isLoading = false
        return
      }
      applications = try await fetchApplications(for: activeZone)
        .sorted { $0.receivedDate > $1.receivedDate }
    } catch {
      handleError(error)
    }
    isLoading = false
  }

  public func selectZone(_ zone: WatchZone) async {
    selectedZone = zone
    selectedStatusFilter = nil
    userDefaults.set(zone.id.value, forKey: zoneSelectionKey)
    isLoading = true
    error = nil
    do {
      applications = try await fetchApplications(for: zone)
        .sorted { $0.receivedDate > $1.receivedDate }
    } catch {
      handleError(error)
    }
    isLoading = false
  }

  private func resolveInitialZone(from zones: [WatchZone]) -> WatchZone? {
    if let savedId = userDefaults.string(forKey: zoneSelectionKey),
       let savedZone = zones.first(where: { $0.id.value == savedId }) {
      return savedZone
    }
    return zones.first
  }

  private func fetchApplications(for zone: WatchZone) async throws -> [PlanningApplication] {
    if let offlineRepository {
      return try await offlineRepository.fetchApplications(for: zone).data
    } else if let repository {
      return try await repository.fetchApplications(for: zone)
    }
    return []
  }
```

- [ ] **Step 4: Update the existing caching test**

The test at line 260 (`loadApplications_withWatchZoneRepository_cachesResolvedZoneOnRetry`) will now fail because zones are refreshed on every `loadApplications()` call. This is intentional — we need to pick up zone changes from the Zones tab.

Replace the test (lines 260-275) with:

```swift
  @Test func loadApplications_withWatchZoneRepository_refreshesZonesOnEveryCall() async throws {
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success([.pendingReview])
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([.cambridge])
    let sut = ApplicationListViewModel(
      watchZoneRepository: zoneSpy,
      repository: appSpy
    )

    await sut.loadApplications()
    await sut.loadApplications()

    #expect(zoneSpy.loadAllCallCount == 2)
    #expect(appSpy.fetchApplicationsCalls.count == 2)
  }
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /Users/christy/Dev/town-crier/mobile/ios && swift test 2>&1 | grep -E "passed|failed|FAIL"`
Expected: All tests pass

- [ ] **Step 6: Commit**

```bash
git add mobile/ios/packages/town-crier-presentation/Sources/Features/ApplicationList/ApplicationListViewModel.swift mobile/ios/town-crier-tests/Sources/Features/ApplicationListViewModelTests.swift
git commit -m "feat(ios): add zone selection to ApplicationListViewModel"
```

---

### Task 3: Wire ZonePickerView into ApplicationListView

**Files:**
- Modify: `mobile/ios/packages/town-crier-presentation/Sources/Features/ApplicationList/ApplicationListView.swift`

- [ ] **Step 1: Add zone picker section to the list**

In `ApplicationListView.swift`, add a `zonePickerSection` computed property and insert it into the list. Modify the `applicationList` computed property (lines 46-61) — add the zone picker section before the filter section:

```swift
  private var applicationList: some View {
    List {
      if viewModel.showZonePicker {
        zonePickerSection
      }

      if viewModel.canFilter {
        filterSection
      }

      ForEach(viewModel.filteredApplications) { application in
        ApplicationListRow(application: application)
          .listRowBackground(Color.tcSurface)
          .contentShape(Rectangle())
          .onTapGesture {
            viewModel.selectApplication(application.id)
          }
      }
    }
    .listStyle(.plain)
  }
```

Add the `zonePickerSection` computed property before the existing `filterSection` (before line 66):

```swift
  // MARK: - Zone Picker

  private var zonePickerSection: some View {
    Section {
      ZonePickerView(
        zones: viewModel.zones,
        selectedZoneId: viewModel.selectedZone?.id
      ) { zone in
        Task {
          await viewModel.selectZone(zone)
        }
      }
    }
    .listRowInsets(EdgeInsets())
    .listRowBackground(Color.tcBackground)
  }
```

- [ ] **Step 2: Verify build**

Run: `cd /Users/christy/Dev/town-crier/mobile/ios && swift build 2>&1 | tail -5`
Expected: Build succeeded

- [ ] **Step 3: Commit**

```bash
git add mobile/ios/packages/town-crier-presentation/Sources/Features/ApplicationList/ApplicationListView.swift
git commit -m "feat(ios): integrate zone picker into ApplicationListView"
```

---

### Task 4: MapViewModel — zone selection (TDD)

**Files:**
- Modify: `mobile/ios/packages/town-crier-presentation/Sources/Features/Map/MapViewModel.swift`
- Modify: `mobile/ios/town-crier-tests/Sources/Features/MapViewModelTests.swift`

- [ ] **Step 1: Write failing tests for zone selection**

Add a `makeSUTWithZones` helper and zone selection tests to `MapViewModelTests.swift`. Add after the existing `// MARK: - Zone-based loading` section (after line 278):

```swift
  // MARK: - Zone Selection

  private func makeSUTWithZones(
    zones: [WatchZone] = [.cambridge, .london],
    applications: [PlanningApplication] = [.pendingReview],
    persistedZoneId: String? = nil
  ) -> (MapViewModel, SpyPlanningApplicationRepository, SpyWatchZoneRepository, UserDefaults) {
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success(applications)
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success(zones)
    let defaults = UserDefaults(suiteName: UUID().uuidString)!
    if let persistedZoneId {
      defaults.set(persistedZoneId, forKey: "test.zone")
    }
    let vm = MapViewModel(
      repository: appSpy,
      watchZoneRepository: zoneSpy,
      userDefaults: defaults,
      zoneSelectionKey: "test.zone"
    )
    return (vm, appSpy, zoneSpy, defaults)
  }

  @Test func loadApplications_populatesZones() async {
    let (sut, _, _, _) = makeSUTWithZones()

    await sut.loadApplications()

    #expect(sut.zones.count == 2)
    #expect(sut.zones[0].id == WatchZone.cambridge.id)
    #expect(sut.zones[1].id == WatchZone.london.id)
  }

  @Test func loadApplications_selectsFirstZoneByDefault() async {
    let (sut, appSpy, _, _) = makeSUTWithZones()

    await sut.loadApplications()

    #expect(sut.selectedZone?.id == WatchZone.cambridge.id)
    #expect(appSpy.fetchApplicationsCalls.first?.id == WatchZone.cambridge.id)
  }

  @Test func loadApplications_restoresPersistedZoneSelection() async {
    let (sut, appSpy, _, _) = makeSUTWithZones(persistedZoneId: "zone-002")

    await sut.loadApplications()

    #expect(sut.selectedZone?.id == WatchZone.london.id)
    #expect(appSpy.fetchApplicationsCalls.first?.id == WatchZone.london.id)
    #expect(sut.centreLat == WatchZone.london.centre.latitude)
    #expect(sut.centreLon == WatchZone.london.centre.longitude)
  }

  @Test func loadApplications_fallsBackToFirstZone_whenPersistedZoneDeleted() async {
    let (sut, appSpy, _, _) = makeSUTWithZones(persistedZoneId: "zone-deleted")

    await sut.loadApplications()

    #expect(sut.selectedZone?.id == WatchZone.cambridge.id)
    #expect(appSpy.fetchApplicationsCalls.first?.id == WatchZone.cambridge.id)
  }

  @Test func selectZone_fetchesApplicationsAndUpdatesCentre() async {
    let (sut, appSpy, _, _) = makeSUTWithZones()
    await sut.loadApplications()
    appSpy.fetchApplicationsCalls.removeAll()

    await sut.selectZone(.london)

    #expect(sut.selectedZone?.id == WatchZone.london.id)
    #expect(appSpy.fetchApplicationsCalls.count == 1)
    #expect(appSpy.fetchApplicationsCalls.first?.id == WatchZone.london.id)
    #expect(sut.centreLat == WatchZone.london.centre.latitude)
    #expect(sut.centreLon == WatchZone.london.centre.longitude)
    #expect(sut.radiusMetres == WatchZone.london.radiusMetres)
  }

  @Test func selectZone_persistsSelectionToUserDefaults() async {
    let (sut, _, _, defaults) = makeSUTWithZones()
    await sut.loadApplications()

    await sut.selectZone(.london)

    #expect(defaults.string(forKey: "test.zone") == "zone-002")
  }

  @Test func showZonePicker_trueWhenMultipleZones() async {
    let (sut, _, _, _) = makeSUTWithZones()
    await sut.loadApplications()

    #expect(sut.showZonePicker)
  }

  @Test func showZonePicker_falseWhenSingleZone() async {
    let (sut, _, _, _) = makeSUTWithZones(zones: [.cambridge])
    await sut.loadApplications()

    #expect(!sut.showZonePicker)
  }
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/christy/Dev/town-crier/mobile/ios && swift test 2>&1 | grep -E "FAIL|error:|Build|passed|failed"`
Expected: Compilation errors — `zones`, `selectedZone`, `showZonePicker`, `selectZone`, `userDefaults`, `zoneSelectionKey` don't exist on MapViewModel yet.

- [ ] **Step 3: Implement MapViewModel changes**

Modify `MapViewModel.swift`. Add new properties after `@Published private(set) var hasLoaded` (line 11):

```swift
  @Published private(set) var zones: [WatchZone] = []
  @Published private(set) var selectedZone: WatchZone?
```

Add stored properties after `private var applications` (line 19):

```swift
  private let userDefaults: UserDefaults
  private let zoneSelectionKey: String
```

Add computed property after `isEmpty` (line 23):

```swift
  public var showZonePicker: Bool {
    zones.count > 1
  }
```

Update the init (line 40) to accept new parameters:

```swift
  public init(
    repository: PlanningApplicationRepository,
    watchZoneRepository: WatchZoneRepository,
    userDefaults: UserDefaults = .standard,
    zoneSelectionKey: String = "lastSelectedZone.map"
  ) {
    self.repository = repository
    self.watchZoneRepository = watchZoneRepository
    self.userDefaults = userDefaults
    self.zoneSelectionKey = zoneSelectionKey
  }
```

Replace the existing `loadApplications()` method (lines 45-71) with:

```swift
  public func loadApplications() async {
    isLoading = true
    error = nil
    do {
      let loadedZones = try await watchZoneRepository.loadAll()
      zones = loadedZones
      if selectedZone == nil || !loadedZones.contains(where: { $0.id == selectedZone?.id }) {
        selectedZone = resolveInitialZone(from: loadedZones)
      }
      guard let zone = selectedZone else {
        isLoading = false
        hasLoaded = true
        return
      }

      centreLat = zone.centre.latitude
      centreLon = zone.centre.longitude
      radiusMetres = zone.radiusMetres

      let fetched = try await repository.fetchApplications(for: zone)
      applications = fetched
      annotations = fetched.compactMap { app in
        guard let location = app.location else { return nil }
        return MapAnnotationItem(application: app, coordinate: location)
      }
    } catch {
      handleError(error)
    }
    isLoading = false
    hasLoaded = true
  }

  public func selectZone(_ zone: WatchZone) async {
    selectedZone = zone
    userDefaults.set(zone.id.value, forKey: zoneSelectionKey)
    centreLat = zone.centre.latitude
    centreLon = zone.centre.longitude
    radiusMetres = zone.radiusMetres
    isLoading = true
    error = nil
    do {
      let fetched = try await repository.fetchApplications(for: zone)
      applications = fetched
      annotations = fetched.compactMap { app in
        guard let location = app.location else { return nil }
        return MapAnnotationItem(application: app, coordinate: location)
      }
    } catch {
      handleError(error)
    }
    isLoading = false
  }

  private func resolveInitialZone(from zones: [WatchZone]) -> WatchZone? {
    if let savedId = userDefaults.string(forKey: zoneSelectionKey),
       let savedZone = zones.first(where: { $0.id.value == savedId }) {
      return savedZone
    }
    return zones.first
  }
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/christy/Dev/town-crier/mobile/ios && swift test 2>&1 | grep -E "passed|failed|FAIL"`
Expected: All tests pass

- [ ] **Step 5: Commit**

```bash
git add mobile/ios/packages/town-crier-presentation/Sources/Features/Map/MapViewModel.swift mobile/ios/town-crier-tests/Sources/Features/MapViewModelTests.swift
git commit -m "feat(ios): add zone selection to MapViewModel"
```

---

### Task 5: Wire ZonePickerView into MapView

**Files:**
- Modify: `mobile/ios/packages/town-crier-presentation/Sources/Features/Map/MapView.swift`

- [ ] **Step 1: Add zone picker and update map positioning**

Three changes to `MapView.swift`:

**1. Add state for map position** — add after the `@StateObject` declaration (line 7):

```swift
  @State private var mapPosition: MapCameraPosition = .automatic
```

**2. Wrap the body in a VStack with the zone picker** — replace the body (starting at line 13) with:

```swift
  public var body: some View {
    VStack(spacing: 0) {
      if viewModel.showZonePicker {
        zonePickerSection
      }
      mapBody
    }
    .background(Color.tcBackground)
    .task {
      await viewModel.loadApplications()
      updateMapPosition()
    }
    .onChange(of: viewModel.selectedZone?.id) { _, _ in
      withAnimation {
        updateMapPosition()
      }
    }
    .sheet(
      item: Binding(
        get: { viewModel.selectedApplication },
        set: { _ in viewModel.clearSelection() }
      )
    ) { application in
      ApplicationSummarySheet(application: application)
    }
  }
```

**3. Add helper views and functions** — add before the `mapContent` computed property:

```swift
  // MARK: - Zone Picker

  private var zonePickerSection: some View {
    ZonePickerView(
      zones: viewModel.zones,
      selectedZoneId: viewModel.selectedZone?.id
    ) { zone in
      Task {
        await viewModel.selectZone(zone)
      }
    }
    .background(Color.tcBackground)
  }
```

Extract the existing ZStack content into `mapBody`:

```swift
  private var mapBody: some View {
    ZStack {
      if viewModel.isLoading && !viewModel.hasLoaded {
        mapPlaceholder
      } else if let error = viewModel.error {
        ErrorStateView(error: error) {
          await viewModel.loadApplications()
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(Color.tcBackground)
      } else if viewModel.isEmpty {
        EmptyStateView(
          icon: "map",
          title: "No Applications",
          description: "No planning applications found in your watch zone yet. Check back soon."
        )
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(Color.tcBackground)
      } else {
        mapContent
        if viewModel.isLoading {
          ProgressView()
            .controlSize(.large)
        }
      }
    }
  }
```

**4. Change Map to use position binding** — in `mapContent`, replace `Map(initialPosition: .region(...))` with:

```swift
    Map(position: $mapPosition) {
```

Remove the `initialPosition` parameter entirely — the position is now managed via `$mapPosition`.

**5. Add the position update helper:**

```swift
  private func updateMapPosition() {
    mapPosition = .region(
      MKCoordinateRegion(
        center: CLLocationCoordinate2D(
          latitude: viewModel.centreLat,
          longitude: viewModel.centreLon
        ),
        latitudinalMeters: viewModel.radiusMetres * 2.5,
        longitudinalMeters: viewModel.radiusMetres * 2.5
      )
    )
  }
```

- [ ] **Step 2: Verify build and all tests pass**

Run: `cd /Users/christy/Dev/town-crier/mobile/ios && swift build 2>&1 | tail -5 && swift test 2>&1 | grep -E "passed|failed|FAIL"`
Expected: Build succeeded, all tests pass

- [ ] **Step 3: Commit**

```bash
git add mobile/ios/packages/town-crier-presentation/Sources/Features/Map/MapView.swift
git commit -m "feat(ios): integrate zone picker into MapView"
```
