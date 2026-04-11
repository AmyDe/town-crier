import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("LoginViewModel")
@MainActor
struct LoginViewModelTests {
  private func makeSUT() -> (LoginViewModel, SpyAuthenticationService) {
    let spy = SpyAuthenticationService()
    let sut = LoginViewModel(authService: spy)
    return (sut, spy)
  }

  // MARK: - Initial state

  @Test func init_isNotLoading() {
    let (sut, _) = makeSUT()
    #expect(!sut.isLoading)
  }

  @Test func init_hasNoError() {
    let (sut, _) = makeSUT()
    #expect(sut.error == nil)
  }

  @Test func init_isNotAuthenticated() {
    let (sut, _) = makeSUT()
    #expect(!sut.isAuthenticated)
  }

  @Test func init_hasNoSession() {
    let (sut, _) = makeSUT()
    #expect(sut.session == nil)
  }

  // MARK: - Login

  @Test func login_setsIsLoadingTrue_thenFalse() async {
    let (sut, spy) = makeSUT()
    spy.loginResult = .success(.valid)

    await sut.login()

    #expect(!sut.isLoading)
  }

  @Test func login_setsSession_onSuccess() async {
    let (sut, spy) = makeSUT()
    spy.loginResult = .success(.valid)

    await sut.login()

    #expect(sut.session == .valid)
    #expect(sut.isAuthenticated)
  }

  @Test func login_callsAuthServiceLogin() async {
    let (sut, spy) = makeSUT()
    spy.loginResult = .success(.valid)

    await sut.login()

    #expect(spy.loginCallCount == 1)
  }

  @Test func login_setsError_onFailure() async {
    let (sut, spy) = makeSUT()
    spy.loginResult = .failure(DomainError.authenticationFailed("cancelled"))

    await sut.login()

    #expect(sut.error == .authenticationFailed("cancelled"))
    #expect(!sut.isAuthenticated)
  }

  @Test func login_clearsError_beforeAttempt() async {
    let (sut, spy) = makeSUT()
    // First login fails
    spy.loginResult = .failure(DomainError.authenticationFailed("fail"))
    await sut.login()
    #expect(sut.error != nil)

    // Second login succeeds
    spy.loginResult = .success(.valid)
    await sut.login()

    #expect(sut.error == nil)
  }

  // MARK: - Logout

  @Test func logout_clearsSession() async {
    let (sut, spy) = makeSUT()
    spy.loginResult = .success(.valid)
    await sut.login()
    #expect(sut.isAuthenticated)

    await sut.logout()

    #expect(!sut.isAuthenticated)
    #expect(sut.session == nil)
  }

  @Test func logout_callsAuthServiceLogout() async {
    let (sut, spy) = makeSUT()
    spy.loginResult = .success(.valid)
    await sut.login()

    await sut.logout()

    #expect(spy.logoutCallCount == 1)
  }

  @Test func logout_setsError_onFailure() async {
    let (sut, spy) = makeSUT()
    spy.loginResult = .success(.valid)
    await sut.login()
    spy.logoutResult = .failure(DomainError.logoutFailed("network"))

    await sut.logout()

    #expect(sut.error == .logoutFailed("network"))
  }

  // MARK: - Check existing session

  @Test func checkExistingSession_setsSession_whenAvailable() async {
    let (sut, spy) = makeSUT()
    spy.currentSessionResult = .valid

    await sut.checkExistingSession()

    #expect(sut.isAuthenticated)
    #expect(sut.session == .valid)
  }

  @Test func checkExistingSession_remainsUnauthenticated_whenNoSession() async {
    let (sut, spy) = makeSUT()
    spy.currentSessionResult = nil

    await sut.checkExistingSession()

    #expect(!sut.isAuthenticated)
  }

  @Test func checkExistingSession_refreshesExpiredSession() async {
    let (sut, spy) = makeSUT()
    spy.currentSessionResult = .expired
    spy.refreshSessionResult = .success(.valid)

    await sut.checkExistingSession()

    #expect(spy.refreshSessionCallCount == 1)
    #expect(sut.session == .valid)
    #expect(sut.isAuthenticated)
  }

  @Test func checkExistingSession_clearsSession_whenRefreshFails() async {
    let (sut, spy) = makeSUT()
    spy.currentSessionResult = .expired
    spy.refreshSessionResult = .failure(DomainError.sessionExpired)

    await sut.checkExistingSession()

    #expect(!sut.isAuthenticated)
    #expect(sut.session == nil)
  }
}
