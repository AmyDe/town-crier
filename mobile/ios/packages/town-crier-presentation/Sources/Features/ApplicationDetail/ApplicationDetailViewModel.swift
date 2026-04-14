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
@MainActor
public final class ApplicationDetailViewModel: ObservableObject {
  public let description: String
  public let address: String
  public let reference: String
  public let authorityName: String
  public let receivedDateFormatted: String
  public let statusLabel: String
  public let statusIcon: String
  public let status: ApplicationStatus
  public let portalUrl: URL?
  public let timelineItems: [TimelineItem]

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

  private let applicationId: PlanningApplicationId
  private let savedApplicationRepository: SavedApplicationRepository?

  public init(
    application: PlanningApplication,
    savedApplicationRepository: SavedApplicationRepository? = nil,
    isSaved: Bool = false
  ) {
    self.applicationId = application.id
    self.savedApplicationRepository = savedApplicationRepository
    self.isSaved = isSaved
    description = application.description
    address = application.address
    reference = application.reference.value
    authorityName = application.authority.name
    receivedDateFormatted = application.receivedDate.formattedForDisplay
    status = application.status
    portalUrl = application.portalUrl

    statusLabel = application.status.displayLabel
    statusIcon = application.status.displayIcon

    let events =
      application.statusHistory.isEmpty
      ? [StatusEvent(status: application.status, date: application.receivedDate)]
      : application.statusHistory.sorted()

    timelineItems = events.enumerated().map { index, event in
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
        try await repository.save(applicationUid: applicationId.value)
        isSaved = true
      } catch {
        // Preserve current state on failure
      }
    }
  }
}
