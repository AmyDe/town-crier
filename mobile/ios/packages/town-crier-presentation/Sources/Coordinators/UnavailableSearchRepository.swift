import TownCrierDomain

/// Stand-in repository used when the Search tab is constructed without a real
/// ``SearchRepository``. Returns an empty result rather than crashing so the
/// empty-state renders cleanly. The composition root in `TownCrierApp` always
/// injects a real repository — this is purely defensive for tests and preview
/// environments.
struct UnavailableSearchRepository: SearchRepository {
  func search(query: String, authorityId: Int, page: Int) async throws -> SearchResult {
    SearchResult(applications: [], total: 0, page: page)
  }
}
