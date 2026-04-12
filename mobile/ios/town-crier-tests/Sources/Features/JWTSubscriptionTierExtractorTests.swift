import Foundation
import Testing

@testable import TownCrierData
@testable import TownCrierDomain

@Suite("JWTSubscriptionTierExtractor")
struct JWTSubscriptionTierExtractorTests {

  // Base64url-encoded JWT header: {"alg": "RS256", "typ": "JWT"}
  private static let header = "eyJhbGciOiAiUlMyNTYiLCAidHlwIjogIkpXVCJ9"

  // MARK: - Valid tier extraction

  @Test func extractTier_returnsPersonal_whenClaimIsPersonal() {
    // JWT payload: {"subscription_tier": "personal", "sub": "auth0|123"}
    let jwt =
      Self.header
      + ".eyJzdWJzY3JpcHRpb25fdGllciI6ICJwZXJzb25hbCIsICJzdWIiOiAiYXV0aDB8MTIzIn0"
      + ".fakesignature"

    let tier = JWTSubscriptionTierExtractor.extractTier(from: jwt)

    #expect(tier == .personal)
  }

  @Test func extractTier_returnsPro_whenClaimIsPro() {
    // JWT payload: {"subscription_tier": "pro", "sub": "auth0|123"}
    let jwt =
      Self.header
      + ".eyJzdWJzY3JpcHRpb25fdGllciI6ICJwcm8iLCAic3ViIjogImF1dGgwfDEyMyJ9"
      + ".fakesignature"

    let tier = JWTSubscriptionTierExtractor.extractTier(from: jwt)

    #expect(tier == .pro)
  }

  // MARK: - Missing or invalid claim defaults to free

  @Test func extractTier_returnsFree_whenClaimIsAbsent() {
    // JWT payload: {"sub": "auth0|123"} -- no subscription_tier
    let jwt =
      Self.header
      + ".eyJzdWIiOiAiYXV0aDB8MTIzIn0"
      + ".fakesignature"

    let tier = JWTSubscriptionTierExtractor.extractTier(from: jwt)

    #expect(tier == .free)
  }

  @Test func extractTier_returnsFree_whenClaimIsUnrecognised() {
    // JWT payload: {"subscription_tier": "enterprise", "sub": "auth0|123"}
    let jwt =
      Self.header
      + ".eyJzdWJzY3JpcHRpb25fdGllciI6ICJlbnRlcnByaXNlIiwgInN1YiI6ICJhdXRoMHwxMjMifQ"
      + ".fakesignature"

    let tier = JWTSubscriptionTierExtractor.extractTier(from: jwt)

    #expect(tier == .free)
  }

  // MARK: - Malformed tokens default to free

  @Test func extractTier_returnsFree_whenTokenIsEmpty() {
    let tier = JWTSubscriptionTierExtractor.extractTier(from: "")
    #expect(tier == .free)
  }

  @Test func extractTier_returnsFree_whenTokenHasNoSegments() {
    let tier = JWTSubscriptionTierExtractor.extractTier(from: "not-a-jwt")
    #expect(tier == .free)
  }

  @Test func extractTier_returnsFree_whenPayloadIsInvalidBase64() {
    let tier = JWTSubscriptionTierExtractor.extractTier(from: "header.!!!invalid!!!.signature")
    #expect(tier == .free)
  }

  // MARK: - Subject (userId) extraction

  @Test func extractSubject_returnsSubClaim_forAuth0User() {
    // JWT payload: {"sub": "auth0|abc123", "email": "user@example.com"}
    let jwt =
      Self.header
      + ".eyJzdWIiOiAiYXV0aDB8YWJjMTIzIiwgImVtYWlsIjogInVzZXJAZXhhbXBsZS5jb20ifQ"
      + ".fakesignature"

    let subject = JWTSubscriptionTierExtractor.extractSubject(from: jwt)

    #expect(subject == "auth0|abc123")
  }

  @Test func extractSubject_returnsSubClaim_forGoogleUser() {
    // JWT payload: {"sub": "google-oauth2|112233", "email": "user@gmail.com"}
    let jwt =
      Self.header
      + ".eyJzdWIiOiAiZ29vZ2xlLW9hdXRoMnwxMTIyMzMiLCAiZW1haWwiOiAidXNlckBnbWFpbC5jb20ifQ"
      + ".fakesignature"

    let subject = JWTSubscriptionTierExtractor.extractSubject(from: jwt)

    #expect(subject == "google-oauth2|112233")
  }

  @Test func extractSubject_returnsSubClaim_forAppleUser() {
    // JWT payload: {"sub": "apple|001122", "email": "user@privaterelay.appleid.com"}
    let jwt =
      Self.header
      + ".eyJzdWIiOiAiYXBwbGV8MDAxMTIyIiwgImVtYWlsIjogInVzZXJAcHJpdmF0ZXJlbGF5LmFwcGxlaWQuY29tIn0"
      + ".fakesignature"

    let subject = JWTSubscriptionTierExtractor.extractSubject(from: jwt)

    #expect(subject == "apple|001122")
  }

  @Test func extractSubject_returnsNil_whenSubClaimIsMissing() {
    // JWT payload: {"email": "user@example.com"}
    let jwt =
      Self.header
      + ".eyJlbWFpbCI6ICJ1c2VyQGV4YW1wbGUuY29tIn0"
      + ".fakesignature"

    let subject = JWTSubscriptionTierExtractor.extractSubject(from: jwt)

    #expect(subject == nil)
  }

  @Test func extractSubject_returnsNil_whenTokenIsMalformed() {
    let subject = JWTSubscriptionTierExtractor.extractSubject(from: "not-a-jwt")

    #expect(subject == nil)
  }

  @Test func extractSubject_returnsNil_whenTokenIsEmpty() {
    let subject = JWTSubscriptionTierExtractor.extractSubject(from: "")

    #expect(subject == nil)
  }
}
