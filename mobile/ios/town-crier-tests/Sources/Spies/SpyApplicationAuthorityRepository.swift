import TownCrierDomain

final class SpyApplicationAuthorityRepository: ApplicationAuthorityRepository, @unchecked Sendable {
  private(set) var fetchAuthoritiesCallCount = 0
  var fetchAuthoritiesResult: Result<ApplicationAuthorityResult, Error> = .success(
    ApplicationAuthorityResult(authorities: [], count: 0)
  )

  func fetchAuthorities() async throws -> ApplicationAuthorityResult {
    fetchAuthoritiesCallCount += 1
    return try fetchAuthoritiesResult.get()
  }
}
