import Testing
import TownCrierDomain

@Suite("ApplicationAuthorityRepository protocol conformance")
struct ApplicationAuthorityRepositoryProtocolTests {

  @Test("SpyApplicationAuthorityRepository conforms to protocol")
  func spyConformsToProtocol() async throws {
    let spy: any ApplicationAuthorityRepository = SpyApplicationAuthorityRepository()

    let result = try await spy.fetchAuthorities()

    #expect(result.authorities.isEmpty)
    #expect(result.count == .zero)
  }

  @Test("SpyApplicationAuthorityRepository records fetch calls")
  func spyRecordsCalls() async throws {
    let spy = SpyApplicationAuthorityRepository()

    _ = try await spy.fetchAuthorities()

    #expect(spy.fetchAuthoritiesCallCount == 1)
  }

  @Test("SpyApplicationAuthorityRepository returns configured result")
  func spyReturnsConfiguredResult() async throws {
    let spy = SpyApplicationAuthorityRepository()
    let expected = ApplicationAuthorityResult(
      authorities: [
        LocalAuthority(code: "123", name: "Bath and NE Somerset", areaType: "Unitary")
      ],
      count: 1
    )
    spy.fetchAuthoritiesResult = .success(expected)

    let result = try await spy.fetchAuthorities()

    #expect(result.authorities.count == 1)
    #expect(result.authorities[0].name == "Bath and NE Somerset")
    #expect(result.authorities[0].areaType == "Unitary")
    #expect(result.count == 1)
  }

  @Test("SpyApplicationAuthorityRepository throws configured error")
  func spyThrowsConfiguredError() async {
    let spy = SpyApplicationAuthorityRepository()
    spy.fetchAuthoritiesResult = .failure(DomainError.networkUnavailable)

    await #expect(throws: DomainError.networkUnavailable) {
      _ = try await spy.fetchAuthorities()
    }
  }
}
