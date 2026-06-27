import Foundation
import Testing

@testable import TownCrierData

/// Covers the pure audience-matching seam that guards the cached-credential
/// read paths after an API environment flip (prod build -> dev build, #660).
/// A cached but still-unexpired access token carries the previous build's
/// audience; the new API 401s it. `audienceMatches` lets the service detect
/// the mismatch and force a fresh login.
@Suite("Auth0AuthenticationService.audienceMatches")
struct Auth0AudienceMatchTests {

  // Base64url-encoded JWT header: {"alg": "RS256", "typ": "JWT"}
  private static let header = "eyJhbGciOiAiUlMyNTYiLCAidHlwIjogIkpXVCJ9"

  private static let devAudience = "https://api-dev.towncrierapp.uk"
  private static let prodAudience = "https://api.towncrierapp.uk"

  private static func jwt(payloadBase64url: String) -> String {
    header + "." + payloadBase64url + ".fakesignature"
  }

  // MARK: - String aud claim

  @Test func audienceMatches_returnsTrue_whenStringAudEqualsExpected() {
    // JWT payload: {"aud": "https://api-dev.towncrierapp.uk"}
    let token = Self.jwt(
      payloadBase64url: "eyJhdWQiOiAiaHR0cHM6Ly9hcGktZGV2LnRvd25jcmllcmFwcC51ayJ9"
    )

    #expect(
      Auth0AuthenticationService.audienceMatches(
        accessToken: token, expected: Self.devAudience
      )
    )
  }

  @Test func audienceMatches_returnsFalse_whenStringAudDiffersFromExpected() {
    // JWT payload: {"aud": "https://api.towncrierapp.uk"}
    let token = Self.jwt(
      payloadBase64url: "eyJhdWQiOiAiaHR0cHM6Ly9hcGkudG93bmNyaWVyYXBwLnVrIn0"
    )

    #expect(
      !Auth0AuthenticationService.audienceMatches(
        accessToken: token, expected: Self.devAudience
      )
    )
  }

  // MARK: - Array aud claim

  @Test func audienceMatches_returnsTrue_whenArrayAudContainsExpected() {
    // JWT payload:
    // {"aud": ["https://api-dev.towncrierapp.uk",
    //          "https://towncrier.eu.auth0.com/userinfo"]}
    let token = Self.jwt(
      payloadBase64url:
        "eyJhdWQiOiBbImh0dHBzOi8vYXBpLWRldi50b3duY3JpZXJhcHAudWsiLCAiaHR0cHM6Ly90b3du"
        + "Y3JpZXIuZXUuYXV0aDAuY29tL3VzZXJpbmZvIl19"
    )

    #expect(
      Auth0AuthenticationService.audienceMatches(
        accessToken: token, expected: Self.devAudience
      )
    )
  }

  @Test func audienceMatches_returnsFalse_whenArrayAudOmitsExpected() {
    // JWT payload:
    // {"aud": ["https://api.towncrierapp.uk",
    //          "https://towncrier.eu.auth0.com/userinfo"]}
    let token = Self.jwt(
      payloadBase64url:
        "eyJhdWQiOiBbImh0dHBzOi8vYXBpLnRvd25jcmllcmFwcC51ayIsICJodHRwczovL3Rvd25j"
        + "cmllci5ldS5hdXRoMC5jb20vdXNlcmluZm8iXX0"
    )

    #expect(
      !Auth0AuthenticationService.audienceMatches(
        accessToken: token, expected: Self.devAudience
      )
    )
  }

  // MARK: - Fail-open on decode failure or missing claim

  @Test func audienceMatches_returnsTrue_whenTokenIsMalformed() {
    #expect(
      Auth0AuthenticationService.audienceMatches(
        accessToken: "not-a-jwt", expected: Self.devAudience
      )
    )
  }

  @Test func audienceMatches_returnsTrue_whenAudClaimIsAbsent() {
    // JWT payload: {"sub": "auth0|123"} -- no aud claim
    let token = Self.jwt(payloadBase64url: "eyJzdWIiOiAiYXV0aDB8MTIzIn0")

    #expect(
      Auth0AuthenticationService.audienceMatches(
        accessToken: token, expected: Self.devAudience
      )
    )
  }
}
