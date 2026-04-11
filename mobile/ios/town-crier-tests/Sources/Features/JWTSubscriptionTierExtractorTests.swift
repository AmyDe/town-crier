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
    let jwt = Self.header
      + ".eyJzdWJzY3JpcHRpb25fdGllciI6ICJwZXJzb25hbCIsICJzdWIiOiAiYXV0aDB8MTIzIn0"
      + ".fakesignature"

    let tier = JWTSubscriptionTierExtractor.extractTier(from: jwt)

    #expect(tier == .personal)
  }

  @Test func extractTier_returnsPro_whenClaimIsPro() {
    // JWT payload: {"subscription_tier": "pro", "sub": "auth0|123"}
    let jwt = Self.header
      + ".eyJzdWJzY3JpcHRpb25fdGllciI6ICJwcm8iLCAic3ViIjogImF1dGgwfDEyMyJ9"
      + ".fakesignature"

    let tier = JWTSubscriptionTierExtractor.extractTier(from: jwt)

    #expect(tier == .pro)
  }

  // MARK: - Missing or invalid claim defaults to free

  @Test func extractTier_returnsFree_whenClaimIsAbsent() {
    // JWT payload: {"sub": "auth0|123"} -- no subscription_tier
    let jwt = Self.header
      + ".eyJzdWIiOiAiYXV0aDB8MTIzIn0"
      + ".fakesignature"

    let tier = JWTSubscriptionTierExtractor.extractTier(from: jwt)

    #expect(tier == .free)
  }

  @Test func extractTier_returnsFree_whenClaimIsUnrecognised() {
    // JWT payload: {"subscription_tier": "enterprise", "sub": "auth0|123"}
    let jwt = Self.header
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
}
