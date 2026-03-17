import Auth0
import Foundation
import TownCrierDomain

/// Auth0 SDK implementation of the authentication service port.
/// Handles login via Universal Login, token storage in Keychain,
/// and session refresh using refresh tokens.
public final class Auth0AuthenticationService: TownCrierDomain.AuthenticationService, @unchecked Sendable {
    private let credentialsManager: CredentialsManager

    public init() {
        let authentication = Auth0.authentication()
        credentialsManager = CredentialsManager(authentication: authentication)
    }

    public func login() async throws -> AuthSession {
        do {
            let credentials = try await Auth0
                .webAuth()
                .scope("openid profile email offline_access")
                .start()

            _ = credentialsManager.store(credentials: credentials)

            return mapToSession(credentials)
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
            try await Auth0.webAuth().clearSession()
            _ = credentialsManager.clear()
        } catch {
            throw DomainError.logoutFailed(error.localizedDescription)
        }
    }

    public func refreshSession() async throws -> AuthSession {
        do {
            let credentials = try await credentialsManager.credentials()
            return mapToSession(credentials)
        } catch {
            _ = credentialsManager.clear()
            throw DomainError.sessionExpired
        }
    }

    public func deleteAccount() async throws {
        do {
            try await Auth0.webAuth().clearSession()
            _ = credentialsManager.clear()
        } catch {
            throw DomainError.logoutFailed(error.localizedDescription)
        }
    }

    public func currentSession() async -> AuthSession? {
        guard credentialsManager.hasValid() else {
            return nil
        }

        do {
            let credentials = try await credentialsManager.credentials()
            return mapToSession(credentials)
        } catch {
            return nil
        }
    }

    private func mapToSession(_ credentials: Credentials) -> AuthSession {
        let profile = UserProfile(
            userId: credentials.idToken,
            email: extractEmail(from: credentials) ?? "",
            name: extractName(from: credentials)
        )

        return AuthSession(
            accessToken: credentials.accessToken,
            idToken: credentials.idToken,
            expiresAt: credentials.expiresIn,
            userProfile: profile
        )
    }

    private func extractEmail(from credentials: Credentials) -> String? {
        guard let jwt = decode(jwt: credentials.idToken) else { return nil }
        return jwt["email"] as? String
    }

    private func extractName(from credentials: Credentials) -> String? {
        guard let jwt = decode(jwt: credentials.idToken) else { return nil }
        return jwt["name"] as? String
    }

    private func decode(jwt token: String) -> [String: Any]? {
        let segments = token.split(separator: ".")
        guard segments.count >= 2 else { return nil }

        var base64 = String(segments[1])
            .replacingOccurrences(of: "-", with: "+")
            .replacingOccurrences(of: "_", with: "/")

        let remainder = base64.count % 4
        if remainder > 0 {
            base64 += String(repeating: "=", count: 4 - remainder)
        }

        guard let data = Data(base64Encoded: base64) else { return nil }
        return try? JSONSerialization.jsonObject(with: data) as? [String: Any]
    }
}
