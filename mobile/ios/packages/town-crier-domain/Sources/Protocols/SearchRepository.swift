/// Port for searching planning applications by keyword within an authority.
///
/// Gated by `Entitlement.searchApplications` -- only Pro tier users may search.
public protocol SearchRepository: Sendable {
  func search(query: String, authorityId: Int, page: Int) async throws -> SearchResult
}
