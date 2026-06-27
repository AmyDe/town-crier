import Auth0
import Foundation
import SimpleKeychain
import TownCrierDomain

/// Auth0 SDK implementation of the authentication service port.
/// Handles login via Universal Login, token storage in Keychain,
/// and session refresh using refresh tokens.
public struct Auth0Config: Sendable {
  public let clientId: String
  public let domain: String
  public let audience: String

  public init(clientId: String, domain: String, audience: String) {
    precondition(!audience.isEmpty, "Auth0Config.audience must not be empty")
    self.clientId = clientId
    self.domain = domain
    self.audience = audience
  }
}

public final class Auth0AuthenticationService: TownCrierDomain.AuthenticationService,
  @unchecked Sendable {
  private let config: Auth0Config
  private let credentialsManager: CredentialsManager
  /// In-memory session cache that lets a foreground burst share a single
  /// keychain read. See `SessionCache` for the contention rationale
  /// (tc-3d7b).
  private let sessionCache = SessionCache()

  public init(config: Auth0Config) {
    self.config = config
    let authentication = Auth0.authentication(
      clientId: config.clientId,
      domain: config.domain
    )
    let keychain = SimpleKeychain(service: "uk.towncrierapp.auth0")
    credentialsManager = CredentialsManager(authentication: authentication, storage: keychain)
  }

  public func login() async throws -> AuthSession {
    do {
      let credentials =
        try await Auth0
        .webAuth(clientId: config.clientId, domain: config.domain)
        .scope("openid profile email offline_access")
        .audience(config.audience)
        .start()

      _ = credentialsManager.store(credentials: credentials)
      let session = Self.mapToSession(credentials)
      await sessionCache.store(session)
      return session
    } catch let error as WebAuthError {
      if case .userCancelled = error {
        throw DomainError.authenticationFailed("cancelled")
      }
      throw DomainError.authenticationFailed(error.localizedDescription)
    } catch {
      throw DomainError.authenticationFailed(error.localizedDescription)
    }
  }

  public func logout() async throws {
    do {
      try await Auth0.webAuth(clientId: config.clientId, domain: config.domain).clearSession()
      _ = credentialsManager.clear()
      await sessionCache.clear()
    } catch {
      throw DomainError.logoutFailed(error.localizedDescription)
    }
  }

  public func refreshSession() async throws -> AuthSession {
    do {
      let credentials = try await credentialsManager.credentials()
      guard Self.audienceMatches(
        accessToken: credentials.accessToken, expected: config.audience
      ) else {
        // The stored token was minted for a different API audience (a
        // prod-build -> dev-build flip, #660). It is still un-expired, so
        // handing it back would 401 against the new API. Wipe it and force
        // a fresh login instead (tc-iuvu).
        _ = credentialsManager.clear()
        await sessionCache.clear()
        throw DomainError.sessionExpired
      }
      let session = Self.mapToSession(credentials)
      await sessionCache.store(session)
      return session
    } catch let error as CredentialsManagerError {
      if Self.isUnrecoverable(error) {
        _ = credentialsManager.clear()
        await sessionCache.clear()
      }
      throw DomainError.sessionExpired
    } catch {
      throw DomainError.sessionExpired
    }
  }

  public func deleteAccount() async throws {
    do {
      try await Auth0.webAuth(clientId: config.clientId, domain: config.domain).clearSession()
      _ = credentialsManager.clear()
      await sessionCache.clear()
    } catch {
      throw DomainError.logoutFailed(error.localizedDescription)
    }
  }

  public func currentSession() async -> AuthSession? {
    // Fast path: in-memory cache. Most foreground bursts hit this and skip
    // securityd entirely (tc-3d7b).
    if let cached = await sessionCache.current() {
      return cached
    }

    // Slow path: consult the keychain. `currentOrLoad` deduplicates
    // concurrent callers so a four-way burst with a cold cache still
    // issues at most one `SecItemCopyMatching`.
    return await sessionCache.currentOrLoad { [credentialsManager, config] in
      // `hasValid()` only checks access-token expiry. Per Auth0 SDK docs,
      // apps using refresh tokens must also consult `canRenew()` at
      // startup — otherwise an expired access token forces a fresh login
      // even though the refresh token is still valid (tc-funq).
      guard credentialsManager.canRenew() || credentialsManager.hasValid() else {
        return nil
      }
      do {
        let credentials = try await credentialsManager.credentials()
        guard Self.audienceMatches(
          accessToken: credentials.accessToken, expected: config.audience
        ) else {
          // Stored token targets a different API audience (env flip, #660).
          // Discard it; returning nil also nulls the session cache via
          // `currentOrLoad`, so the next call re-authenticates (tc-iuvu).
          _ = credentialsManager.clear()
          return nil
        }
        return Self.mapToSession(credentials)
      } catch {
        return nil
      }
    }
  }

  /// Whether a credentials-manager failure means the refresh token is
  /// permanently unusable. Only these failures justify wiping the keychain;
  /// transient errors (network, biometrics, store failures) must not force
  /// the user to re-authenticate.
  private static func isUnrecoverable(_ error: CredentialsManagerError) -> Bool {
    switch error {
    case .noRefreshToken, .noCredentials:
      return true
    case .renewFailed, .apiExchangeFailed, .ssoExchangeFailed:
      if let cause = error.cause as? AuthenticationError {
        return cause.isInvalidRefreshToken || cause.isRefreshTokenDeleted
      }
      return false
    default:
      return false
    }
  }

  /// Whether the access token was issued for the API audience this build
  /// targets. After an environment flip (prod build -> dev build, #660) a
  /// cached but un-expired token still carries the previous audience, which
  /// the new API rejects with 401. Comparing the `aud` claim against
  /// `config.audience` lets the cached-credential read paths detect the
  /// mismatch and force a fresh login.
  ///
  /// The `aud` claim may be a string or an array of strings; both are
  /// matched by membership. **Fails open**: a token that cannot be decoded
  /// or carries no `aud` claim is treated as matching, so a malformed-token
  /// hiccup never wipes an otherwise valid session.
  static func audienceMatches(accessToken: String, expected: String) -> Bool {
    let audiences = JWTSubscriptionTierExtractor.extractAudiences(from: accessToken)
    guard !audiences.isEmpty else { return true }
    return audiences.contains(expected)
  }

  private static func mapToSession(_ credentials: Credentials) -> AuthSession {
    let profile = UserProfile(
      userId: extractUserId(from: credentials) ?? credentials.idToken,
      email: extractEmail(from: credentials) ?? "",
      name: extractName(from: credentials)
    )

    let tier = JWTSubscriptionTierExtractor.extractTier(
      from: credentials.accessToken
    )

    return AuthSession(
      accessToken: credentials.accessToken,
      idToken: credentials.idToken,
      expiresAt: credentials.expiresIn,
      userProfile: profile,
      subscriptionTier: tier
    )
  }

  private static func extractUserId(from credentials: Credentials) -> String? {
    JWTSubscriptionTierExtractor.extractSubject(from: credentials.idToken)
  }

  private static func extractEmail(from credentials: Credentials) -> String? {
    guard let jwt = JWTSubscriptionTierExtractor.decodePayload(from: credentials.idToken)
    else { return nil }
    return jwt["email"] as? String
  }

  private static func extractName(from credentials: Credentials) -> String? {
    guard let jwt = JWTSubscriptionTierExtractor.decodePayload(from: credentials.idToken)
    else { return nil }
    return jwt["name"] as? String
  }
}
