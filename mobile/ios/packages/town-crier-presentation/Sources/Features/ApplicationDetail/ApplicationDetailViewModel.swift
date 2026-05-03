import Foundation
import TownCrierDomain

/// A single display-ready item in the status timeline.
public struct TimelineItem: Equatable, Sendable {
  public let label: String
  public let icon: String
  public let dateFormatted: String
  public let isCurrent: Bool
  public let status: ApplicationStatus
}

/// ViewModel exposing display-ready properties for a planning application detail screen.
///
/// Optionally supports save/unsave when a ``SavedApplicationRepository`` is injected.
///
/// Optionally supports stale-while-revalidate refresh when a
/// ``PlanningApplicationRepository`` is injected. The view presents the cached
/// payload synchronously, then ``refresh()`` runs the per-id fetch in the
/// background. The fetch must still fire on every detail open because
/// `GetApplicationByUidQueryHandler.TryRefreshSavedSnapshotAsync` (bd tc-udby)
/// is the only path that keeps saved-row snapshots fresh on the server.
@MainActor
public final class ApplicationDetailViewModel: ObservableObject {
  /// The current planning application payload. Display-ready properties are
  /// computed over this value so a successful ``refresh()`` updates the UI in
  /// place. Marked `@Published` so SwiftUI re-renders when the payload changes.
  @Published public private(set) var application: PlanningApplication

  public var description: String { application.description }
  public var address: String { application.address }
  public var reference: String { application.reference.value }
  public var authorityName: String { application.authority.name }
  public var receivedDateFormatted: String { application.receivedDate.formattedForDisplay }
  public var status: ApplicationStatus { application.status }
  public var portalUrl: URL? { application.portalUrl }

  public var statusLabel: String { application.status.displayLabel }
  public var statusIcon: String { application.status.displayIcon }

  public var timelineItems: [TimelineItem] {
    let events =
      application.statusHistory.isEmpty
      ? [StatusEvent(status: application.status, date: application.receivedDate)]
      : application.statusHistory.sorted()

    return events.enumerated().map { index, event in
      let isLast = index == events.count - 1
      return TimelineItem(
        label: event.status.displayLabel,
        icon: event.status.displayIcon,
        dateFormatted: event.date.formattedForDisplay,
        isCurrent: isLast,
        status: event.status
      )
    }
  }

  /// Whether the application is currently saved/bookmarked by the user.
  @Published public private(set) var isSaved: Bool

  public var onOpenPortal: ((URL) -> Void)?
  public var onDismiss: (() -> Void)?

  /// Whether the save/unsave action is available (repository was provided).
  public var canSave: Bool {
    savedApplicationRepository != nil
  }

  public var hasPortalUrl: Bool {
    portalUrl != nil
  }

  public var hasTimeline: Bool {
    !timelineItems.isEmpty
  }

  private let savedApplicationRepository: SavedApplicationRepository?
  private let planningApplicationRepository: PlanningApplicationRepository?

  private var applicationId: PlanningApplicationId { application.id }

  public init(
    application: PlanningApplication,
    savedApplicationRepository: SavedApplicationRepository? = nil,
    planningApplicationRepository: PlanningApplicationRepository? = nil,
    isSaved: Bool = false
  ) {
    self.application = application
    self.savedApplicationRepository = savedApplicationRepository
    self.planningApplicationRepository = planningApplicationRepository
    self.isSaved = isSaved
  }

  /// Loads the saved state from the repository and updates `isSaved` accordingly.
  /// Call this after creating the ViewModel to reflect the server-side saved state.
  /// No-op if no repository was provided.
  public func loadSavedState() async {
    guard let repository = savedApplicationRepository else { return }
    do {
      let saved = try await repository.loadAll()
      isSaved = saved.contains { $0.applicationUid == applicationId.value }
    } catch {
      // Leave isSaved at its current value (false) on failure
    }
  }

  /// Re-fetches the application by id and replaces the cached payload on
  /// success. Silent on failure — the cached payload remains visible. Required
  /// to keep saved-row snapshots fresh on the server even though the sheet
  /// presents the cached payload synchronously (bd tc-sslz, tc-udby).
  /// No-op if no `PlanningApplicationRepository` was injected.
  public func refresh() async {
    guard let repository = planningApplicationRepository else { return }
    do {
      let fresh = try await repository.fetchApplication(by: applicationId)
      if fresh != application {
        application = fresh
      }
    } catch {
      // Silent on failure — keep the cached payload visible.
    }
  }

  public func openPortal() {
    guard let url = portalUrl else { return }
    onOpenPortal?(url)
  }

  public func dismiss() {
    onDismiss?()
  }

  /// Toggles the saved state, calling the repository to persist the change.
  /// No-op if no repository was provided.
  public func toggleSave() async {
    guard let repository = savedApplicationRepository else { return }

    if isSaved {
      do {
        try await repository.remove(applicationUid: applicationId.value)
        isSaved = false
      } catch {
        // Preserve current state on failure
      }
    } else {
      do {
        try await repository.save(application: application)
        isSaved = true
      } catch {
        // Preserve current state on failure
      }
    }
  }
}
