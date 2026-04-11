import TownCrierDomain

final class SpySearchRepository: SearchRepository, @unchecked Sendable {
  struct SearchCall: Equatable {
    let query: String
    let authorityId: Int
    let page: Int
  }

  private(set) var searchCalls: [SearchCall] = []
  var searchResult: Result<SearchResult, Error> = .success(
    SearchResult(applications: [], total: 0, page: 1)
  )

  func search(query: String, authorityId: Int, page: Int) async throws -> SearchResult {
    searchCalls.append(SearchCall(query: query, authorityId: authorityId, page: page))
    return try searchResult.get()
  }
}
