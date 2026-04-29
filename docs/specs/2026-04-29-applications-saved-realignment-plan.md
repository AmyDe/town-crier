# Applications / Saved Realignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Separate Applications and Saved into independent surfaces on both iOS and web, eliminating the 'All' zone chip dead-end.

**Architecture:** Two-tab/two-page model per platform. Applications stays per-zone; Saved is a flat cross-zone list (including orphan saves). All status filtering moves to free for all tiers. No backend changes — both endpoints already exist.

**Tech Stack:** Swift / SwiftUI / Swift Testing (iOS); React / TypeScript / Vitest / Testing Library (web).

**Spec:** `docs/specs/applications-saved-realignment.md`

---

## Task 1: iOS — `SavedApplicationListViewModel`

**Files:**
- Create: `mobile/ios/packages/town-crier-presentation/Sources/Features/SavedApplicationList/SavedApplicationListViewModel.swift`
- Create: `mobile/ios/town-crier-tests/Sources/Features/SavedApplicationListViewModelTests.swift`

The new VM. Loads via `SavedApplicationRepository.loadAll()`, sorts by `SavedApplication.savedAt` desc, exposes a status filter that's free for all tiers.

- [ ] **Step 1.1: Write failing tests**

```swift
// SavedApplicationListViewModelTests.swift
import Testing
import Foundation
@testable import TownCrierDomain
@testable import TownCrierPresentation

@MainActor
struct SavedApplicationListViewModelTests {
  @Test func loadsAndSortsBySavedAtDesc() async throws {
    let older = SavedApplication.fixture(uid: "A", savedAt: Date(timeIntervalSince1970: 1000))
    let newer = SavedApplication.fixture(uid: "B", savedAt: Date(timeIntervalSince1970: 2000))
    let repo = SavedApplicationRepositorySpy(saved: [older, newer])
    let sut = SavedApplicationListViewModel(savedApplicationRepository: repo)

    await sut.loadAll()

    #expect(sut.applications.map { $0.id.value } == ["B", "A"])
    #expect(sut.isEmpty == false)
  }

  @Test func emptyStateWhenNoSaves() async throws {
    let repo = SavedApplicationRepositorySpy(saved: [])
    let sut = SavedApplicationListViewModel(savedApplicationRepository: repo)

    await sut.loadAll()

    #expect(sut.applications.isEmpty)
    #expect(sut.isEmpty)
  }

  @Test func statusFilterPassthrough() async throws {
    let permitted = SavedApplication.fixture(uid: "A", status: .permitted)
    let pending = SavedApplication.fixture(uid: "B", status: .pending)
    let repo = SavedApplicationRepositorySpy(saved: [permitted, pending])
    let sut = SavedApplicationListViewModel(savedApplicationRepository: repo)
    await sut.loadAll()

    sut.selectedStatusFilter = .pending

    #expect(sut.filteredApplications.map { $0.id.value } == ["B"])
  }

  @Test func statusFilterNoMatchesIsEmpty() async throws {
    let permitted = SavedApplication.fixture(uid: "A", status: .permitted)
    let repo = SavedApplicationRepositorySpy(saved: [permitted])
    let sut = SavedApplicationListViewModel(savedApplicationRepository: repo)
    await sut.loadAll()

    sut.selectedStatusFilter = .rejected

    #expect(sut.filteredApplications.isEmpty)
    #expect(sut.isEmpty)
  }

  @Test func repositoryErrorIsCaptured() async throws {
    let repo = SavedApplicationRepositorySpy(error: DomainError.networkUnavailable)
    let sut = SavedApplicationListViewModel(savedApplicationRepository: repo)

    await sut.loadAll()

    #expect(sut.error == .networkUnavailable)
    #expect(sut.applications.isEmpty)
  }
}
```

The `SavedApplicationRepositorySpy` and `SavedApplication.fixture(uid:savedAt:status:)` helper need to exist. Check `mobile/ios/town-crier-tests/Sources/Fixtures/SavedApplication+Fixtures.swift` — extend if needed; do not introduce reflection-based mocks.

- [ ] **Step 1.2: Run tests, verify they fail (compile errors)**

```bash
cd mobile/ios && swift test --filter SavedApplicationListViewModelTests 2>&1 | tail -20
```
Expected: compile error — `SavedApplicationListViewModel` doesn't exist.

- [ ] **Step 1.3: Implement minimal VM**

```swift
// SavedApplicationListViewModel.swift
import Combine
import Foundation
import TownCrierDomain

@MainActor
public final class SavedApplicationListViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published private(set) var applications: [PlanningApplication] = []
  @Published var selectedStatusFilter: ApplicationStatus?
  @Published private(set) var isLoading = false
  @Published var error: DomainError?

  private let savedApplicationRepository: SavedApplicationRepository

  var onApplicationSelected: ((PlanningApplicationId) -> Void)?

  public var filteredApplications: [PlanningApplication] {
    guard let filter = selectedStatusFilter else { return applications }
    return applications.filter { $0.status == filter }
  }

  public var isEmpty: Bool {
    filteredApplications.isEmpty && error == nil && !isLoading
  }

  public init(savedApplicationRepository: SavedApplicationRepository) {
    self.savedApplicationRepository = savedApplicationRepository
  }

  public func loadAll() async {
    isLoading = true
    error = nil
    do {
      let saved = try await savedApplicationRepository.loadAll()
      applications = saved
        .sorted { $0.savedAt > $1.savedAt }
        .compactMap(\.application)
    } catch {
      handleError(error)
    }
    isLoading = false
  }

  public func selectApplication(_ id: PlanningApplicationId) {
    onApplicationSelected?(id)
  }
}
```

- [ ] **Step 1.4: Run tests, verify pass**

```bash
cd mobile/ios && swift test --filter SavedApplicationListViewModelTests 2>&1 | tail -20
```
Expected: all 5 tests pass.

- [ ] **Step 1.5: Lint clean**

```bash
cd mobile/ios && swiftlint lint --strict --quiet
```
Expected: zero warnings.

- [ ] **Step 1.6: Commit**

```bash
git add mobile/ios/packages/town-crier-presentation/Sources/Features/SavedApplicationList/ \
        mobile/ios/town-crier-tests/Sources/Features/SavedApplicationListViewModelTests.swift \
        mobile/ios/town-crier-tests/Sources/Fixtures/SavedApplication+Fixtures.swift
git commit -m "feat(ios): add SavedApplicationListViewModel (<bead-id>)"
```

---

## Task 2: iOS — `SavedApplicationListView`

**Files:**
- Create: `mobile/ios/packages/town-crier-presentation/Sources/Features/SavedApplicationList/SavedApplicationListView.swift`

The view layer. Status pill row + list of `ApplicationListRow` + empty/error/loading states. No new row component — reuse `ApplicationListRow`.

- [ ] **Step 2.1: Implement view**

```swift
// SavedApplicationListView.swift
import SwiftUI
import TownCrierDomain

public struct SavedApplicationListView: View {
  @ObservedObject var viewModel: SavedApplicationListViewModel

  public init(viewModel: SavedApplicationListViewModel) {
    self.viewModel = viewModel
  }

  public var body: some View {
    VStack(spacing: 0) {
      StatusFilterPillRow(selected: $viewModel.selectedStatusFilter)
      content
    }
    .navigationTitle("Saved")
    #if os(iOS)
      .navigationBarTitleDisplayMode(.inline)
    #endif
    .task { await viewModel.loadAll() }
    .refreshable { await viewModel.loadAll() }
  }

  @ViewBuilder
  private var content: some View {
    if viewModel.isLoading && viewModel.applications.isEmpty {
      ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
    } else if let error = viewModel.error {
      ErrorStateView(error: error)
    } else if viewModel.isEmpty {
      EmptyStateView(
        title: viewModel.selectedStatusFilter == nil
          ? "No saved applications yet"
          : "No saved applications match this filter",
        message: viewModel.selectedStatusFilter == nil
          ? "Bookmark applications you want to track. Tap the bookmark icon on any application detail."
          : "Try a different filter."
      )
    } else {
      List(viewModel.filteredApplications) { application in
        ApplicationListRow(application: application)
          .onTapGesture { viewModel.selectApplication(application.id) }
      }
    }
  }
}
```

`StatusFilterPillRow`, `EmptyStateView`, `ErrorStateView`: extract from `ApplicationListView` if not already shared. If currently inline in `ApplicationListView`, move them to `Features/ApplicationList/Components/` as their own files (one component per file) and import here.

- [ ] **Step 2.2: Build and lint**

```bash
cd mobile/ios && swift build && swiftlint lint --strict --quiet
```
Expected: clean build, zero lint warnings.

- [ ] **Step 2.3: Commit**

```bash
git add mobile/ios/packages/town-crier-presentation/Sources/Features/SavedApplicationList/SavedApplicationListView.swift \
        mobile/ios/packages/town-crier-presentation/Sources/Features/ApplicationList/Components/
git commit -m "feat(ios): add SavedApplicationListView (<bead-id>)"
```

---

## Task 3: iOS — Wire Saved tab into `AppCoordinator` + `TownCrierApp`

**Files:**
- Modify: `mobile/ios/packages/town-crier-presentation/Sources/Coordinators/AppCoordinator.swift`
- Modify: `mobile/ios/town-crier-app/Sources/TownCrierApp.swift`
- Modify: `mobile/ios/town-crier-tests/Sources/Features/AppCoordinatorTests.swift`
- Modify: `mobile/ios/town-crier-tests/Sources/Features/CompositionRootTests.swift`

- [ ] **Step 3.1: Add factory test**

In `AppCoordinatorTests.swift`, add:

```swift
@Test func makeSavedApplicationListViewModel_returnsConfiguredVM() {
  let coordinator = AppCoordinator(
    // … existing builder, with savedApplicationRepository injected
  )
  let viewModel = coordinator.makeSavedApplicationListViewModel()
  #expect(viewModel.selectedStatusFilter == nil)
}
```

- [ ] **Step 3.2: Add factory to `AppCoordinator`**

```swift
public func makeSavedApplicationListViewModel() -> SavedApplicationListViewModel {
  guard let repo = savedApplicationRepository else {
    fatalError("savedApplicationRepository must be configured to use the Saved tab")
  }
  let viewModel = SavedApplicationListViewModel(savedApplicationRepository: repo)
  viewModel.onApplicationSelected = { [weak self] id in
    self?.detailApplication = self?.applicationsById[id]
  }
  return viewModel
}
```

(Adjust the tap-through wiring to match the existing `makeApplicationListViewModel` pattern — copy that wiring, do not invent a new one.)

- [ ] **Step 3.3: Add tab in `TownCrierApp.mainTabView`**

In `TownCrierApp.swift`, between the Applications and Map `NavigationStack`s, insert:

```swift
NavigationStack {
  SavedApplicationListView(viewModel: coordinator.makeSavedApplicationListViewModel())
    .id(coordinator.subscriptionTier)
}
.sheet(item: $coordinator.detailApplication) { application in
  NavigationStack {
    ApplicationDetailView(
      viewModel: coordinator.makeApplicationDetailViewModel(application: application)
    )
  }
}
.tabItem {
  Label("Saved", systemImage: "bookmark.fill")
}
```

The detail sheet wiring duplicates the Applications tab's — that's intentional, both lists open the same detail.

- [ ] **Step 3.4: Run tests + build + lint**

```bash
cd mobile/ios && swift test && swiftlint lint --strict --quiet
```
Expected: full suite green.

- [ ] **Step 3.5: Commit**

```bash
git add mobile/ios/packages/town-crier-presentation/Sources/Coordinators/AppCoordinator.swift \
        mobile/ios/town-crier-app/Sources/TownCrierApp.swift \
        mobile/ios/town-crier-tests/Sources/Features/AppCoordinatorTests.swift \
        mobile/ios/town-crier-tests/Sources/Features/CompositionRootTests.swift
git commit -m "feat(ios): wire Saved tab into TownCrierApp (<bead-id>)"
```

---

## Task 4: iOS — Strip 'All' chip, Saved filter, `canFilter` from Applications and Map

**Files:**
- Modify: `mobile/ios/packages/town-crier-presentation/Sources/Features/ApplicationList/ApplicationListViewModel.swift`
- Modify: `mobile/ios/packages/town-crier-presentation/Sources/Features/ApplicationList/ApplicationListView.swift`
- Modify: `mobile/ios/packages/town-crier-presentation/Sources/Features/Map/MapViewModel.swift`
- Modify: `mobile/ios/packages/town-crier-presentation/Sources/Features/Map/MapView.swift`
- Delete: `mobile/ios/town-crier-tests/Sources/Features/ApplicationListAllZonesTests.swift`
- Modify: `mobile/ios/town-crier-tests/Sources/Features/ApplicationListViewModelTests.swift`
- Modify: `mobile/ios/town-crier-tests/Sources/Features/ApplicationListViewModelStaleZoneTests.swift`
- Modify: `mobile/ios/town-crier-tests/Sources/Features/MapViewModelTests.swift`

- [ ] **Step 4.1: Delete `ApplicationListAllZonesTests.swift`**

```bash
rm mobile/ios/town-crier-tests/Sources/Features/ApplicationListAllZonesTests.swift
```

- [ ] **Step 4.2: Strip Saved-related state from `ApplicationListViewModel`**

In `ApplicationListViewModel.swift`, delete the following members:
- `isAllZonesSelected`, `isSavedFilterActive`, `isLoadingSaved`, `savedApplicationUids`
- `static let allZonesSentinel`
- `EmptyStateKind` (the only remaining case becomes redundant)
- `emptyStateKind` computed property
- `selectAllZones()`, `activateSavedFilter()`, `deactivateSavedFilter()`
- `canFilter` computed property
- `savedApplicationRepository` stored property + the parameter from all four initialisers
- The `didSet` mutual-exclusion logic on `selectedStatusFilter` (the property becomes a plain `@Published var`)

Replace `filteredApplications` body:

```swift
public var filteredApplications: [PlanningApplication] {
  guard let filter = selectedStatusFilter else { return applications }
  return applications.filter { $0.status == filter }
}
```

Replace `loadApplications()` body with the per-zone-only flow (delete the `if isAllZonesSelected` early return):

```swift
public func loadApplications() async {
  isLoading = true
  error = nil
  do {
    if let watchZoneRepository {
      let loadedZones = try await watchZoneRepository.loadAll()
      zones = loadedZones
      if let currentId = selectedZone?.id,
         let updated = loadedZones.first(where: { $0.id == currentId }) {
        selectedZone = updated
      } else {
        resolveInitialSelection(from: loadedZones)
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
```

In `selectZone()`, drop `isAllZonesSelected = false` and `isSavedFilterActive = false`.

In `resolveInitialSelection()`, drop the `Self.allZonesSentinel` branch.

Restore `showZonePicker` to the original guard:

```swift
public var showZonePicker: Bool {
  zones.count > 1
}
```

- [ ] **Step 4.3: Strip `canFilter` from `MapViewModel`**

Delete `canFilter` from `MapViewModel`. In `filteredAnnotations`:

```swift
public var filteredAnnotations: [MapAnnotationItem] {
  guard let filter = selectedStatusFilter else { return annotations }
  return annotations.filter { $0.status == filter }
}
```

- [ ] **Step 4.4: Strip 'All' chip + Saved filter from `ApplicationListView`**

In `ApplicationListView.swift`:
- Remove the leading 'All' chip in the zone scroller (the synthetic chip prepended by tc-nb5u).
- Remove the Saved filter pill.
- Replace `if viewModel.canFilter || viewModel.canSave { … }` with `if !viewModel.applications.isEmpty { StatusFilterPillRow(selected: $viewModel.selectedStatusFilter) }` — pills always render when there are applications.
- Remove the `.allZonesNoSavedFilter` branch in the empty-state view (collapses to a single empty state).

In `MapView.swift`: replace `if viewModel.canFilter` with always-render of the pill row.

- [ ] **Step 4.5: Update existing tests**

In `ApplicationListViewModelTests.swift`:
- Delete tests asserting `canFilter == false` paywall behaviour.
- Delete tests covering Saved filter on Applications (`activateSavedFilter`, `deactivateSavedFilter`, `isSavedFilterActive`).
- Update any `showZonePicker` test that expected it to be true with one zone — should be true only with > 1 zone.

In `ApplicationListViewModelStaleZoneTests.swift`: keep the zone-staleness coverage; delete any cases that exercise Saved-cross-zone behaviour.

In `MapViewModelTests.swift`: delete tests asserting `canFilter == false` paywall behaviour.

- [ ] **Step 4.6: Run full iOS suite, build, lint**

```bash
cd mobile/ios && swift test && swiftlint lint --strict --quiet
```
Expected: full suite green; lint clean.

- [ ] **Step 4.7: Commit**

```bash
git add -u mobile/ios/
git commit -m "refactor(ios): drop 'All' chip + Saved filter + canFilter (<bead-id>)"
```

---

## Task 5: Web — `useSavedApplications` hook

**Files:**
- Create: `web/src/features/SavedApplications/useSavedApplications.ts`
- Create: `web/src/features/SavedApplications/__tests__/useSavedApplications.test.ts`

- [ ] **Step 5.1: Write failing tests**

```typescript
// useSavedApplications.test.ts
import { describe, it, expect } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import { useSavedApplications } from '../useSavedApplications';
import { FakeSavedApplicationRepository } from '../../../test-doubles/FakeSavedApplicationRepository';

describe('useSavedApplications', () => {
  it('loads and sorts by savedAt desc', async () => {
    const repo = new FakeSavedApplicationRepository([
      { applicationUid: 'A', savedAt: '2026-01-01T00:00:00Z', application: app('A', 'pending') },
      { applicationUid: 'B', savedAt: '2026-02-01T00:00:00Z', application: app('B', 'permitted') },
    ]);
    const { result } = renderHook(() => useSavedApplications({ savedRepository: repo }));
    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.applications.map((a) => a.uid)).toEqual(['B', 'A']);
  });

  it('returns empty when repository returns nothing', async () => {
    const repo = new FakeSavedApplicationRepository([]);
    const { result } = renderHook(() => useSavedApplications({ savedRepository: repo }));
    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.applications).toEqual([]);
  });

  it('filters by selectedStatusFilter', async () => {
    const repo = new FakeSavedApplicationRepository([
      { applicationUid: 'A', savedAt: '2026-01-01T00:00:00Z', application: app('A', 'pending') },
      { applicationUid: 'B', savedAt: '2026-02-01T00:00:00Z', application: app('B', 'permitted') },
    ]);
    const { result } = renderHook(() => useSavedApplications({ savedRepository: repo }));
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    act(() => result.current.setStatusFilter('pending'));

    expect(result.current.applications.map((a) => a.uid)).toEqual(['A']);
  });

  it('captures repository error', async () => {
    const repo = new FakeSavedApplicationRepository(new Error('Network unavailable'));
    const { result } = renderHook(() => useSavedApplications({ savedRepository: repo }));
    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.error).toBe('Network unavailable');
    expect(result.current.applications).toEqual([]);
  });
});

function app(uid: string, appState: 'pending' | 'permitted' | 'rejected' | 'conditions') {
  return { uid, address: '1 Test St', description: 'desc', appState, /* … fixture fields */ };
}
```

If `FakeSavedApplicationRepository` doesn't exist, create it under `web/src/test-doubles/` mirroring the existing fakes. Hand-written; no `vi.mock()`.

- [ ] **Step 5.2: Run tests, verify they fail**

```bash
cd web && npx vitest run useSavedApplications 2>&1 | tail -20
```
Expected: fail — module not found.

- [ ] **Step 5.3: Implement hook**

```typescript
// useSavedApplications.ts
import { useState, useEffect, useCallback, useMemo } from 'react';
import type {
  PlanningApplicationSummary,
  ApplicationStatus,
} from '../../domain/types';
import type { SavedApplicationRepository } from '../../domain/ports/saved-application-repository';

export interface UseSavedApplicationsOptions {
  readonly savedRepository: SavedApplicationRepository;
}

interface State {
  readonly applications: readonly PlanningApplicationSummary[];
  readonly isLoading: boolean;
  readonly error: string | null;
  readonly selectedStatusFilter: ApplicationStatus | null;
}

const INITIAL: State = {
  applications: [],
  isLoading: true,
  error: null,
  selectedStatusFilter: null,
};

export function useSavedApplications(options: UseSavedApplicationsOptions) {
  const { savedRepository } = options;
  const [state, setState] = useState<State>(INITIAL);

  useEffect(() => {
    let cancelled = false;
    savedRepository
      .listSaved()
      .then((saved) => {
        if (cancelled) return;
        const sorted = [...saved].sort(
          (a, b) => Date.parse(b.savedAt) - Date.parse(a.savedAt),
        );
        setState((prev) => ({
          ...prev,
          applications: sorted.map((s) => s.application),
          isLoading: false,
        }));
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setState((prev) => ({
          ...prev,
          applications: [],
          isLoading: false,
          error: err instanceof Error ? err.message : 'Unknown error',
        }));
      });
    return () => {
      cancelled = true;
    };
  }, [savedRepository]);

  const setStatusFilter = useCallback((status: ApplicationStatus | null) => {
    setState((prev) => ({ ...prev, selectedStatusFilter: status }));
  }, []);

  const filtered = useMemo<readonly PlanningApplicationSummary[]>(() => {
    if (state.selectedStatusFilter === null) return state.applications;
    return state.applications.filter((a) => a.appState === state.selectedStatusFilter);
  }, [state.applications, state.selectedStatusFilter]);

  return {
    applications: filtered,
    isLoading: state.isLoading,
    error: state.error,
    selectedStatusFilter: state.selectedStatusFilter,
    setStatusFilter,
  };
}
```

- [ ] **Step 5.4: Run tests, verify pass**

```bash
cd web && npx vitest run useSavedApplications 2>&1 | tail -20
```
Expected: 4 tests pass.

- [ ] **Step 5.5: Type-check**

```bash
cd web && npx tsc --noEmit
```
Expected: clean.

- [ ] **Step 5.6: Commit**

```bash
git add web/src/features/SavedApplications/ web/src/test-doubles/FakeSavedApplicationRepository.ts
git commit -m "feat(web): add useSavedApplications hook (<bead-id>)"
```

---

## Task 6: Web — `SavedApplicationsPage` view

**Files:**
- Create: `web/src/features/SavedApplications/SavedApplicationsPage.tsx`
- Create: `web/src/features/SavedApplications/SavedApplicationsPage.module.css`
- Create: `web/src/features/SavedApplications/__tests__/SavedApplicationsPage.test.tsx`

- [ ] **Step 6.1: Write failing tests**

```tsx
// SavedApplicationsPage.test.tsx
import { describe, it, expect } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { SavedApplicationsPage } from '../SavedApplicationsPage';
import { FakeSavedApplicationRepository } from '../../../test-doubles/FakeSavedApplicationRepository';

describe('SavedApplicationsPage', () => {
  it('renders saved applications', async () => {
    const repo = new FakeSavedApplicationRepository([
      { applicationUid: 'A', savedAt: '2026-01-01T00:00:00Z', application: { uid: 'A', address: '1 Test St', appState: 'pending', description: 'desc' } },
    ]);
    render(
      <MemoryRouter>
        <SavedApplicationsPage savedRepository={repo} />
      </MemoryRouter>,
    );
    await waitFor(() => expect(screen.getByText('1 Test St')).toBeInTheDocument());
  });

  it('renders empty state when no saves', async () => {
    const repo = new FakeSavedApplicationRepository([]);
    render(
      <MemoryRouter>
        <SavedApplicationsPage savedRepository={repo} />
      </MemoryRouter>,
    );
    await waitFor(() =>
      expect(
        screen.getByText(/Bookmark applications you want to track/i),
      ).toBeInTheDocument(),
    );
  });

  it('filters by status pill', async () => {
    const repo = new FakeSavedApplicationRepository([
      { applicationUid: 'A', savedAt: '2026-01-01T00:00:00Z', application: { uid: 'A', address: '1 Test St', appState: 'pending', description: 'A' } },
      { applicationUid: 'B', savedAt: '2026-02-01T00:00:00Z', application: { uid: 'B', address: '2 Test St', appState: 'permitted', description: 'B' } },
    ]);
    render(
      <MemoryRouter>
        <SavedApplicationsPage savedRepository={repo} />
      </MemoryRouter>,
    );
    await waitFor(() => expect(screen.getByText('1 Test St')).toBeInTheDocument());

    await userEvent.click(screen.getByRole('button', { name: /pending/i }));

    expect(screen.getByText('1 Test St')).toBeInTheDocument();
    expect(screen.queryByText('2 Test St')).not.toBeInTheDocument();
  });
});
```

- [ ] **Step 6.2: Run, verify fail**

```bash
cd web && npx vitest run SavedApplicationsPage
```
Expected: module not found.

- [ ] **Step 6.3: Implement page**

```tsx
// SavedApplicationsPage.tsx
import { Link } from 'react-router-dom';
import { useSavedApplications } from './useSavedApplications';
import { StatusFilterPillRow } from '../../components/StatusFilterPillRow/StatusFilterPillRow';
import type { SavedApplicationRepository } from '../../domain/ports/saved-application-repository';
import styles from './SavedApplicationsPage.module.css';

export interface SavedApplicationsPageProps {
  readonly savedRepository: SavedApplicationRepository;
}

export function SavedApplicationsPage({ savedRepository }: SavedApplicationsPageProps) {
  const { applications, isLoading, error, selectedStatusFilter, setStatusFilter } =
    useSavedApplications({ savedRepository });

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <h1>Saved</h1>
      </header>
      <StatusFilterPillRow
        selected={selectedStatusFilter}
        onChange={setStatusFilter}
      />
      {isLoading ? (
        <div role="status">Loading…</div>
      ) : error ? (
        <div role="alert">{error}</div>
      ) : applications.length === 0 ? (
        <div className={styles.empty}>
          {selectedStatusFilter === null
            ? 'Bookmark applications you want to track. Tap the bookmark icon on any application detail.'
            : 'No saved applications match this filter.'}
        </div>
      ) : (
        <ul className={styles.list}>
          {applications.map((app) => (
            <li key={app.uid}>
              <Link to={`/applications/${app.uid}`}>{app.address}</Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
```

If `StatusFilterPillRow` doesn't already exist as a shared component, extract it from `ApplicationsPage` first into `web/src/components/StatusFilterPillRow/`.

CSS Module uses design-token vars only. No magic colours/spacing.

- [ ] **Step 6.4: Run tests, verify pass + type-check**

```bash
cd web && npx vitest run SavedApplicationsPage && npx tsc --noEmit
```
Expected: 3 tests pass; type-check clean.

- [ ] **Step 6.5: Commit**

```bash
git add web/src/features/SavedApplications/ web/src/components/StatusFilterPillRow/
git commit -m "feat(web): add SavedApplicationsPage (<bead-id>)"
```

---

## Task 7: Web — Connected page, route, sidebar nav

**Files:**
- Create: `web/src/features/SavedApplications/ConnectedSavedApplicationsPage.tsx`
- Modify: `web/src/AppRoutes.tsx`
- Modify: `web/src/components/Sidebar/Sidebar.tsx`
- Modify (if exists): `web/src/components/Sidebar/__tests__/Sidebar.test.tsx`

- [ ] **Step 7.1: Add Connected wrapper**

```tsx
// ConnectedSavedApplicationsPage.tsx
import { useApiClient } from '../../infra/useApiClient';
import { ApiSavedApplicationRepository } from '../../infra/ApiSavedApplicationRepository';
import { SavedApplicationsPage } from './SavedApplicationsPage';
import { useMemo } from 'react';

export function ConnectedSavedApplicationsPage() {
  const client = useApiClient();
  const repo = useMemo(() => new ApiSavedApplicationRepository(client), [client]);
  return <SavedApplicationsPage savedRepository={repo} />;
}
```

(Check the actual DI conventions — match what `ConnectedApplicationsPage` does.)

- [ ] **Step 7.2: Register route**

In `AppRoutes.tsx`, add the lazy-loaded route between `/applications/*` and `/watch-zones`:

```tsx
<Route path="/saved" element={<Suspense fallback={null}><ConnectedSavedApplicationsPage /></Suspense>} />
```

Add the import + `lazy()` wrapper in line with the existing pattern.

- [ ] **Step 7.3: Add Sidebar nav entry**

In `Sidebar.tsx`, the items list:

```typescript
{ label: 'Dashboard', to: '/dashboard' },
{ label: 'Applications', to: '/applications' },
{ label: 'Saved', to: '/saved' },
{ label: 'Watch Zones', to: '/watch-zones' },
{ label: 'Map', to: '/map' },
{ label: 'Search', to: '/search' },
{ label: 'Notifications', to: '/notifications' },
{ label: 'Settings', to: '/settings' },
```

If a `Sidebar.test` exists, add an assertion that the Saved link renders.

- [ ] **Step 7.4: Run tests + type-check**

```bash
cd web && npx vitest run && npx tsc --noEmit
```
Expected: full suite green.

- [ ] **Step 7.5: Commit**

```bash
git add web/src/features/SavedApplications/ConnectedSavedApplicationsPage.tsx \
        web/src/AppRoutes.tsx \
        web/src/components/Sidebar/
git commit -m "feat(web): wire /saved route and sidebar nav (<bead-id>)"
```

---

## Task 8: Web — Strip Saved filter from `useApplications` and `ApplicationsPage`

**Files:**
- Modify: `web/src/features/Applications/useApplications.ts`
- Modify: `web/src/features/Applications/ApplicationsPage.tsx`
- Modify: `web/src/features/Applications/__tests__/useApplications.test.ts` (if exists)
- Modify: `web/src/features/Applications/__tests__/ApplicationsPage.test.tsx`

- [ ] **Step 8.1: Strip Saved-related state from `useApplications`**

In `useApplications.ts`, delete:
- `selectAllZones`, `isAllZonesSelected`, `activateSavedFilter`, `deactivateSavedFilter`, `isSavedFilterActive`, `savedUids` (state, returned values, callbacks).
- The `savedRepository` parameter from `UseApplicationsOptions` and the destructure.
- The cross-zone payload-from-saved branch in `activateSavedFilter` (entire function deleted).
- The `isSavedFilterActive` clears in `selectZone`.

Update `setStatusFilter` to drop the saved-mutual-exclusion branch:

```typescript
const setStatusFilter = useCallback((status: ApplicationStatus | null) => {
  setState((prev) => ({ ...prev, selectedStatusFilter: status }));
}, []);
```

Update `filteredApplications` to status-filter-only:

```typescript
const filteredApplications = useMemo(() => {
  if (state.selectedStatusFilter === null) return state.applications;
  return state.applications.filter((a) => a.appState === state.selectedStatusFilter);
}, [state.applications, state.selectedStatusFilter]);
```

- [ ] **Step 8.2: Strip Saved chip from `ApplicationsPage`**

In `ApplicationsPage.tsx`, remove the Saved chip JSX and any 'All' zone UI / selectAllZones invocations.

- [ ] **Step 8.3: Update existing tests**

In `useApplications.test.ts`: delete cases covering Saved filter, `selectAllZones`, `isAllZonesSelected`. Keep the per-zone fetch + status filter cases.

In `ApplicationsPage.test.tsx`: delete Saved chip + 'All' chip cases.

- [ ] **Step 8.4: Run tests + type-check**

```bash
cd web && npx vitest run && npx tsc --noEmit
```
Expected: full suite green.

- [ ] **Step 8.5: Commit**

```bash
git add -u web/
git commit -m "refactor(web): drop Saved filter and 'All' chip from /applications (<bead-id>)"
```

---

## Task 9: Bookkeeping — close `tc-34cg`

The 'All' zone dead-end bug is no longer reachable once Tasks 1–4 ship.

- [ ] **Step 9.1: Close the bead**

```bash
bd close tc-34cg
```

Add a closing note explaining supersession:

```bash
bd update tc-34cg --notes "Superseded by Applications/Saved realignment. The 'All' zone chip is gone; the dead-end interaction no longer exists."
```

(If `bd close` does not accept notes, run the `--notes` update first then close.)

---

## Self-Review

- **Spec coverage:** Tier matrix (Task 4 + Task 5/6/8) ✓; iOS Saved tab (Tasks 1-3) ✓; iOS Applications cleanup (Task 4) ✓; Map cleanup (Task 4) ✓; Web Saved page (Tasks 5-7) ✓; Web Applications cleanup (Task 8) ✓; Bookkeeping (Task 9) ✓; Out-of-scope (notifications, backend, Map zone selector, Search, Dashboard) untouched ✓.
- **Placeholders:** none. Every code block contains the actual code an engineer needs.
- **Type consistency:** `SavedApplicationListViewModel` initialiser shape matches between Task 1 (definition) and Task 3 (usage in coordinator). `useSavedApplications` return shape matches between Task 5 (definition) and Task 6 (consumption).
- **Bead split:** Tasks 1–4 → one iOS bead; Tasks 5–8 → one web bead; Task 9 → bookkeeping. iOS and web ship independently. Each platform's add-and-remove ships in a single PR (no transient duplicate-Saved state in production).
- **Worker assignment:** iOS bead → `ios-tdd-worker`. Web bead → `react-tdd-worker`. Bookkeeping → no worker; manual or run by orchestrator.

---

## Execution Notes

- Each platform bead is one autopilot session. The worker creates a worktree, runs through its tasks in order, ships one PR.
- Per the project bead-first / worktree-first rules: worktrees are created by the orchestrator, the worker enters and works inside; worker commits include `(<bead-id>)` in the subject for `bd doctor` orphan detection.
- The shared `StatusFilterPillRow` extraction is a small refactor that may need to land in either Task 2 (iOS) or Task 6 (web) depending on whether the component already exists there. If it doesn't, factor it out as the first step of those tasks.
