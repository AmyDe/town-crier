package uk.towncrierapp.domain.auth

/**
 * Hand-written fake for [AuthenticationService], shared (via `:domain`'s
 * `testFixtures` source set) by `:data`'s `ApiClient` tests and
 * `:presentation`'s auth-coordinator/ViewModel tests — see testing.md's
 * cross-module fake-sharing convention.
 */
public class FakeAuthenticationService(
    public var currentSessionResult: AuthSession? = anAuthSession(),
) : AuthenticationService {
    public var loginResult: Result<AuthSession> = Result.success(anAuthSession())
    public var refreshSessionResult: Result<AuthSession> = Result.success(anAuthSession())

    public val loginCalls: MutableList<Unit> = mutableListOf()
    public val logoutCalls: MutableList<Unit> = mutableListOf()
    public val refreshSessionCalls: MutableList<Unit> = mutableListOf()

    override suspend fun login(): AuthSession {
        loginCalls += Unit
        return loginResult.getOrThrow()
    }

    override suspend fun logout() {
        logoutCalls += Unit
    }

    override suspend fun refreshSession(): AuthSession {
        refreshSessionCalls += Unit
        return refreshSessionResult.getOrThrow()
    }

    override suspend fun currentSession(): AuthSession? = currentSessionResult
}
