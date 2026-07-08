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

  /// Canonical public share URL for this application (GH #738 Slice 4), built
  /// from the API-supplied `authoritySlug` and the full PlanIt ref. `nil` when
  /// the authority carries no slug (e.g. the cached list payload before
  /// ``refresh()`` refetches by-id), so the view never offers a broken,
  /// slug-less link.
  public var shareURL: URL? {
    guard let slug = application.authority.slug else { return nil }
    return ShareURL.build(authoritySlug: slug, ref: application.id.name)
  }

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
  /// Fired only on a successful false→true save (never on unsave or failure).
  /// Drives the review-prompt `savedApplication` signal (GH #628).
  public var onSaved: (() -> Void)?
  /// Fired by the anonymous sign-up CTA (GH#879 Phase 2), in place of the
  /// Save affordance. Wired by the coordinator to the app's single Auth0
  /// entry point, the same one every other sign-up/sign-in surface uses.
  public var onRequestSignUp: (() -> Void)?

  /// Whether the save/unsave action is available (repository was provided).
  public var canSave: Bool {
    savedApplicationRepository != nil
  }

  /// Whether the anonymous sign-up CTA should replace the Save affordance
  /// (GH#879 Phase 2) — true exactly when this view model was built for the
  /// anonymous detail path (an ``AnonymousApplicationDetailRepository`` was
  /// injected instead of the authed save/refresh repositories).
  public var showsSignUpCTA: Bool {
    anonymousApplicationDetailRepository != nil
  }

  public var hasPortalUrl: Bool {
    portalUrl != nil
  }

  public var hasTimeline: Bool {
    !timelineItems.isEmpty
  }

  private let savedApplicationRepository: SavedApplicationRepository?
  private let planningApplicationRepository: PlanningApplicationRepository?
  /// The anonymous (no-session) by-slug refresh path (GH#879 Phase 2).
  /// Mutually driving with `planningApplicationRepository` in ``refresh()``:
  /// when present, it takes precedence — a view model is only ever built for
  /// one path or the other by the coordinator's factory.
  private let anonymousApplicationDetailRepository: AnonymousApplicationDetailRepository?

  /// Re-entrancy guard for ``refresh()``. The detail sheet's `.task` can fire
  /// repeatedly for a single open — alongside scenePhase changes or a
  /// re-appear — and each unguarded call previously spawned a duplicate
  /// per-id fetch. SRE telemetry caught `GET /v1/applications/{ref}` firing up
  /// to 6 times within seconds, all cancelled (HTTP 499). While one refresh is
  /// in flight, further calls short-circuit (bd tc-eum5).
  private var isRefreshInFlight = false

  private var applicationId: PlanningApplicationId { application.id }

  public init(
    application: PlanningApplication,
    savedApplicationRepository: SavedApplicationRepository? = nil,
    planningApplicationRepository: PlanningApplicationRepository? = nil,
    anonymousApplicationDetailRepository: AnonymousApplicationDetailRepository? = nil,
    isSaved: Bool = false
  ) {
    self.application = application
    self.savedApplicationRepository = savedApplicationRepository
    self.planningApplicationRepository = planningApplicationRepository
    self.anonymousApplicationDetailRepository = anonymousApplicationDetailRepository
    self.isSaved = isSaved
  }

  /// Loads the saved state from the repository and updates `isSaved` accordingly.
  /// Call this after creating the ViewModel to reflect the server-side saved state.
  /// No-op if no repository was provided.
  ///
  /// The match compares the reconstructed ``PlanningApplicationId`` of each
  /// saved item's embedded snapshot against this application's id — never the
  /// raw top-level `applicationUid` string. The stored top-level uid can be in
  /// a different format from the snapshot id reconstructed by
  /// `PlanningApplicationDTO.toDomain()`, which would otherwise make a saved
  /// item read as "unsaved" (bd tc-jjl4). The snapshot id is the source of
  /// truth the rest of the Saved tab already uses.
  public func loadSavedState() async {
    guard let repository = savedApplicationRepository else { return }
    do {
      let saved = try await repository.loadAll()
      isSaved = saved.contains { $0.application?.id == application.id }
    } catch {
      // Leave isSaved at its current value (false) on failure
    }
  }

  /// Re-fetches the application and replaces the cached payload on success.
  /// Silent on failure — the cached payload remains visible.
  ///
  /// Two mutually-exclusive paths, chosen by which repository the
  /// coordinator's factory injected: the authed by-id read (required to keep
  /// saved-row snapshots fresh on the server, bd tc-sslz/tc-udby), or the
  /// anonymous by-slug read (GH#879 Phase 2), which is skipped silently when
  /// the cached application carries no authority slug — refreshing a
  /// slug-less payload by slug is meaningless. No-op if neither repository
  /// was injected.
  public func refresh() async {
    // Drop re-entrant calls while a refresh is already running so a single
    // detail open issues at most one fetch (bd tc-eum5). The flag is read
    // and set synchronously before any `await`, so a second call scheduled
    // on the same actor sees it set and returns immediately.
    guard !isRefreshInFlight else { return }
    isRefreshInFlight = true
    defer { isRefreshInFlight = false }

    if let anonymousApplicationDetailRepository {
      guard let slug = application.authority.slug else { return }
      do {
        let fresh = try await anonymousApplicationDetailRepository.fetchApplication(
          bySlug: slug, ref: application.id.name)
        if fresh != application {
          application = fresh
        }
      } catch {
        // Silent on failure — keep the cached payload visible.
      }
      return
    }

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

  /// Invokes the anonymous sign-up CTA (GH#879 Phase 2).
  public func requestSignUp() {
    onRequestSignUp?()
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
        onSaved?()
      } catch {
        // Preserve current state on failure
      }
    }
  }
}
