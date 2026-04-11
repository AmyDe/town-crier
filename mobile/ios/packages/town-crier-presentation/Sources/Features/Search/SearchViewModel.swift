import Combine
import Foundation
import TownCrierDomain

/// ViewModel driving the search feature with authority selector and pagination.
///
/// Search is Pro-gated: free and personal tier users are proactively shown the
/// subscription upsell sheet instead of executing a search. If the server returns
/// a 403 `insufficient_entitlement`, the reactive fallback also triggers the upsell.
@MainActor
public final class SearchViewModel: ObservableObject, EntitlementGatingViewModel {
  @Published private(set) var applications: [PlanningApplication] = []
  @Published private(set) var isLoading = false
  @Published private(set) var total = 0
  @Published var error: DomainError?
  @Published var entitlementGate: Entitlement?
  @Published var query = ""
  @Published var selectedAuthorityId: Int?

  /// Whether a search has been performed (used to distinguish "no results" from "not yet searched").
  @Published private(set) var hasSearched = false

  private let repository: SearchRepository
  private let featureGate: FeatureGate
  private var currentPage = 1
  private var lastPageHadMore = false

  var onApplicationSelected: ((PlanningApplicationId) -> Void)?

  /// Whether the user's tier allows searching.
  public var isSearchEnabled: Bool {
    featureGate.hasEntitlement(.searchApplications)
  }

  /// Whether more pages of results are available.
  public var hasMore: Bool {
    lastPageHadMore
  }

  /// Whether the search returned zero results after a completed search.
  public var isEmpty: Bool {
    hasSearched && applications.isEmpty && error == nil && !isLoading
  }

  public init(
    repository: SearchRepository,
    featureGate: FeatureGate
  ) {
    self.repository = repository
    self.featureGate = featureGate
  }

  /// Performs a new search, resetting pagination.
  public func search() async {
    let trimmedQuery = query.trimmingCharacters(in: .whitespaces)
    guard !trimmedQuery.isEmpty, let authorityId = selectedAuthorityId else {
      return
    }

    // Proactive gating: free/personal users get upsell instead of search
    guard isSearchEnabled else {
      entitlementGate = .searchApplications
      return
    }

    isLoading = true
    error = nil
    applications = []
    currentPage = 1
    lastPageHadMore = false
    hasSearched = true

    do {
      let result = try await repository.search(
        query: trimmedQuery,
        authorityId: authorityId,
        page: 1
      )
      applications = result.applications
      total = result.total
      lastPageHadMore = result.hasMore
    } catch {
      handleError(error)
    }
    isLoading = false
  }

  /// Loads the next page of results, appending to the current list.
  public func loadMore() async {
    guard hasMore, let authorityId = selectedAuthorityId else { return }

    let trimmedQuery = query.trimmingCharacters(in: .whitespaces)
    guard !trimmedQuery.isEmpty else { return }

    let nextPage = currentPage + 1
    isLoading = true

    do {
      let result = try await repository.search(
        query: trimmedQuery,
        authorityId: authorityId,
        page: nextPage
      )
      applications.append(contentsOf: result.applications)
      total = result.total
      currentPage = nextPage
      lastPageHadMore = result.hasMore
    } catch {
      handleError(error)
    }
    isLoading = false
  }

  /// Notifies the coordinator that the user selected an application.
  public func selectApplication(_ id: PlanningApplicationId) {
    onApplicationSelected?(id)
  }
}
