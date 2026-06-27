import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Tests for the haversine-distance sort mode on `ApplicationListViewModel`
/// (tc-mso6). Mirrors the web sibling tc-ge7j so the two clients behave
/// identically: ordering by ascending haversine distance from the active
/// watch zone's centre to each application's `location`, with apps that
/// have no location sorted last.
///
/// Spec: tc-1nsa.8 / tc-1nsa.11. Split from `ApplicationListViewModelUnreadTests`
/// to stay under SwiftLint's `file_length` ceiling.
@Suite("ApplicationListViewModel — distance sort (tc-mso6)")
@MainActor
struct ApplicationListViewModelDistanceSortTests {

  private func makeSUT(
    applications: [PlanningApplication] = [],
    sortKey: String = "test.distanceSort"
  ) throws -> (
    ApplicationListViewModel,
    SpyPlanningApplicationRepository,
    UserDefaults
  ) {
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success(applications)
    let stateSpy = SpyNotificationStateRepository()
    stateSpy.fetchStateResult = .success(
      NotificationState(
        lastReadAt: Date(timeIntervalSince1970: 0),
        version: 1,
        totalUnreadCount: 0
      )
    )
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    let sut = ApplicationListViewModel(
      repository: appSpy,
      zone: .cambridge,
      notificationStateRepository: stateSpy,
      userDefaults: defaults,
      sortKey: sortKey
    )
    return (sut, appSpy, defaults)
  }

  // MARK: - Enum surface

  @Test("ApplicationsSort.distance has stable raw value mirroring web sibling")
  func sort_distance_rawValueMatchesWeb() {
    #expect(ApplicationsSort.distance.rawValue == "distance")
  }

  @Test("ApplicationsSort.distance has a user-facing label")
  func sort_distance_displayLabel() {
    #expect(ApplicationsSort.distance.displayLabel == "Distance")
  }

  // MARK: - Sort behaviour (server-driven since GH#682 slice 1)

  @Test("distance sort preserves the server order — no local haversine re-sort")
  func sort_distance_preservesServerOrder() async throws {
    // Distance is now server-driven (KNN nearest-first) and paged via infinite
    // scroll, so the client must not re-order locally — that would only sort the
    // pages already loaded. The ordering and NULLS-LAST behaviour are proven by
    // the Go pgtest suite (#688); the client just renders what it receives. The
    // deliberately non-distance order below proves the client leaves it intact.
    let serverOrdered: [PlanningApplication] = [.rejected, .permitted, .pendingReview]
    let (sut, _, _) = try makeSUT(applications: serverOrdered)

    await sut.loadApplications()
    sut.sort = .distance

    #expect(sut.filteredApplications.map(\.id) == serverOrdered.map(\.id))
  }

  // MARK: - Persistence

  @Test("setting sort to distance persists 'distance' to UserDefaults")
  func setSort_distance_persistsToDefaults() throws {
    let (sut, _, defaults) = try makeSUT(sortKey: "persist.distance")

    sut.sort = .distance

    #expect(defaults.string(forKey: "persist.distance") == "distance")
  }

  @Test("ViewModel restores persisted distance sort on init")
  func sort_distance_restoredFromDefaults() throws {
    let appSpy = SpyPlanningApplicationRepository()
    let stateSpy = SpyNotificationStateRepository()
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    defaults.set("distance", forKey: "restore.distance")

    let sut = ApplicationListViewModel(
      repository: appSpy,
      zone: .cambridge,
      notificationStateRepository: stateSpy,
      userDefaults: defaults,
      sortKey: "restore.distance"
    )

    #expect(sut.sort == .distance)
  }

  // MARK: - Picker visibility (multi-zone "no zone" guard)

  @Test("availableSortOptions includes .distance when a zone is active")
  func availableSortOptions_includesDistance_whenZoneActive() async throws {
    let (sut, _, _) = try makeSUT()
    await sut.loadApplications()
    #expect(sut.availableSortOptions.contains(.distance))
  }

  @Test("availableSortOptions hides .distance when no zone is selected")
  func availableSortOptions_hidesDistance_whenNoZoneSelected() async throws {
    let appSpy = SpyPlanningApplicationRepository()
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([])
    let stateSpy = SpyNotificationStateRepository()
    stateSpy.fetchStateResult = .success(
      NotificationState(
        lastReadAt: Date(timeIntervalSince1970: 0),
        version: 1,
        totalUnreadCount: 0
      )
    )
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    let sut = ApplicationListViewModel(
      watchZoneRepository: zoneSpy,
      repository: appSpy,
      notificationStateRepository: stateSpy,
      userDefaults: defaults,
      zoneSelectionKey: "no-zone.zone",
      sortKey: "no-zone.sort"
    )

    await sut.loadApplications()

    #expect(!sut.availableSortOptions.contains(.distance))
  }

  @Test("availableSortOptions retains all non-distance modes")
  func availableSortOptions_retainsOtherModes() throws {
    let (sut, _, _) = try makeSUT()
    let modes = sut.availableSortOptions
    #expect(modes.contains(.recentActivity))
    #expect(modes.contains(.newest))
    #expect(modes.contains(.oldest))
    #expect(modes.contains(.status))
  }
}
