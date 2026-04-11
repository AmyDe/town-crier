import Foundation
import TownCrierDomain

/// ViewModel driving the saved applications list.
///
/// Saved applications are available to all tiers -- no entitlement gating required.
/// Loads the full list from the API and supports optimistic removal on unsave.
@MainActor
public final class SavedApplicationsViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published private(set) var savedApplications: [SavedApplication] = []
  @Published private(set) var isLoading = false
  @Published var error: DomainError?

  /// Whether a load has been performed (distinguishes "no results" from "not yet loaded").
  @Published private(set) var hasLoaded = false

  private let repository: SavedApplicationRepository

  /// Callback invoked when the user taps a saved application to view its detail.
  public var onApplicationSelected: ((String) -> Void)?

  /// Whether the load returned zero results after a completed load.
  public var isEmpty: Bool {
    hasLoaded && savedApplications.isEmpty && error == nil && !isLoading
  }

  public init(repository: SavedApplicationRepository) {
    self.repository = repository
  }

  /// Loads or refreshes the saved applications list.
  public func loadSavedApplications() async {
    isLoading = true
    error = nil
    savedApplications = []
    hasLoaded = true

    do {
      savedApplications = try await repository.loadAll()
    } catch {
      handleError(error)
    }
    isLoading = false
  }

  /// Removes a saved application by UID, optimistically updating the local list.
  public func unsave(applicationUid: String) async {
    do {
      try await repository.remove(applicationUid: applicationUid)
      savedApplications.removeAll { $0.applicationUid == applicationUid }
    } catch {
      handleError(error)
    }
  }

  /// Notifies the coordinator that the user wants to view a saved application's detail.
  public func selectApplication(uid: String) {
    onApplicationSelected?(uid)
  }
}
