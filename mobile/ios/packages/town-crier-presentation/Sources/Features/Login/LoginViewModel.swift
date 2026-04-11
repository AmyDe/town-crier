import Combine
import TownCrierDomain

/// ViewModel managing login, logout, and session restoration.
@MainActor
public final class LoginViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published public private(set) var isLoading = false
  @Published public internal(set) var error: DomainError?
  @Published public private(set) var session: AuthSession?

  private let authService: AuthenticationService

  public var isAuthenticated: Bool {
    session != nil
  }

  public init(authService: AuthenticationService) {
    self.authService = authService
  }

  /// Presents the Auth0 login/registration UI.
  public func login() async {
    isLoading = true
    error = nil
    do {
      session = try await authService.login()
    } catch {
      handleError(error) { .authenticationFailed($0) }
    }
    isLoading = false
  }

  /// Clears the current session.
  public func logout() async {
    error = nil
    do {
      try await authService.logout()
      session = nil
    } catch {
      handleError(error) { .logoutFailed($0) }
    }
  }

  /// Checks for an existing stored session and refreshes if expired.
  public func checkExistingSession() async {
    guard let existing = await authService.currentSession() else {
      return
    }

    if existing.isExpired {
      do {
        session = try await authService.refreshSession()
      } catch {
        session = nil
      }
    } else {
      session = existing
    }
  }
}
