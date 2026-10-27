import Combine
import Foundation
import TownCrierDomain

/// Saved applications sorted by `savedAt` descending. Saves without a
/// denormalised `application` payload are dropped.
@MainActor
public final class SavedApplicationListViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published private(set) var applications: [PlanningApplication] = []
  @Published var selectedStatusFilter: ApplicationStatus?
  @Published private(set) var isLoading = false
  @Published var error: DomainError?

  private let savedApplicationRepository: SavedApplicationRepository

  var onApplicationSelected: ((PlanningApplicationId) -> Void)?

  /// Carries the full row payload so the detail sheet can be presented synchronously.
  var onApplicationSelectedWithPayload: ((PlanningApplication) -> Void)?

  public var filteredApplications: [PlanningApplication] {
    guard let filter = selectedStatusFilter else { return applications }
    return applications.filter { $0.status == filter }
  }

  /// True when there is nothing to render and the list is neither loading nor
  /// in error: no saves at all, or the active filter excludes every save.
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
      applications =
        saved
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

  public func selectApplication(_ application: PlanningApplication) {
    onApplicationSelectedWithPayload?(application)
  }
}
