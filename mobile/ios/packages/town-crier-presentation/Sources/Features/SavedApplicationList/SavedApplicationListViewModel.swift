import Combine
import Foundation
import TownCrierDomain

/// ViewModel for the dedicated Saved tab — a flat, cross-zone feed of the
/// user's bookmarked planning applications, sorted by `savedAt` descending so
/// the most recently bookmarked items appear first. The denormalised
/// `SavedApplication.application` payload supplies row data without an N+1
/// fetch; saves without a payload (legacy entries) are dropped.
///
/// The status filter is free for every subscription tier — paywall lives at
/// the watch-zone level, not on personal bookmarks.
@MainActor
public final class SavedApplicationListViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published private(set) var applications: [PlanningApplication] = []
  @Published var selectedStatusFilter: ApplicationStatus?
  @Published private(set) var isLoading = false
  @Published var error: DomainError?

  private let savedApplicationRepository: SavedApplicationRepository

  var onApplicationSelected: ((PlanningApplicationId) -> Void)?

  /// Payload-bearing selection callback used by the Saved tab so the detail
  /// sheet can be presented synchronously from the cached row data; the detail
  /// view model then runs `refresh()` in the background to keep the saved-row
  /// snapshot fresh on the server (bd tc-sslz, tc-udby).
  var onApplicationSelectedWithPayload: ((PlanningApplication) -> Void)?

  public var filteredApplications: [PlanningApplication] {
    guard let filter = selectedStatusFilter else { return applications }
    return applications.filter { $0.status == filter }
  }

  /// True when the list has nothing to render right now and we are not in a
  /// loading or error state — either no saves at all, or the active filter
  /// excludes every save. The view picks copy by inspecting
  /// `selectedStatusFilter`.
  public var isEmpty: Bool {
    filteredApplications.isEmpty && error == nil && !isLoading
  }

  public var isNetworkError: Bool {
    error == .networkUnavailable
  }

  public var isServerError: Bool {
    if case .serverError = error { return true }
    return false
  }

  public var isSessionExpired: Bool {
    error == .sessionExpired
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
      applications = []
    }
    isLoading = false
  }

  public func selectApplication(_ id: PlanningApplicationId) {
    onApplicationSelected?(id)
  }

  /// Payload-bearing selection used by the Saved tab — the row already has the
  /// full `PlanningApplication` so the detail sheet opens instantly. The
  /// detail view model fires the per-id refresh in the background.
  public func selectApplication(_ application: PlanningApplication) {
    onApplicationSelectedWithPayload?(application)
  }
}
