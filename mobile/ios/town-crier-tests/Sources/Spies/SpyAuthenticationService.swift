import Foundation
import TownCrierDomain

final class SpyAuthenticationService: AuthenticationService, @unchecked Sendable {
    private(set) var loginCallCount = 0
    var loginResult: Result<AuthSession, Error> = .success(.valid)

    func login() async throws -> AuthSession {
        loginCallCount += 1
        return try loginResult.get()
    }

    private(set) var logoutCallCount = 0
    var logoutResult: Result<Void, Error> = .success(())

    func logout() async throws {
        logoutCallCount += 1
        try logoutResult.get()
    }

    private(set) var refreshSessionCallCount = 0
    var refreshSessionResult: Result<AuthSession, Error> = .success(.valid)

    func refreshSession() async throws -> AuthSession {
        refreshSessionCallCount += 1
        return try refreshSessionResult.get()
    }

    private(set) var currentSessionCallCount = 0
    var currentSessionResult: AuthSession?

    func currentSession() async -> AuthSession? {
        currentSessionCallCount += 1
        return currentSessionResult
    }

    private(set) var deleteAccountCallCount = 0
    var deleteAccountResult: Result<Void, Error> = .success(())

    func deleteAccount() async throws {
        deleteAccountCallCount += 1
        try deleteAccountResult.get()
    }
}
