import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("ErrorHandlingViewModel")
@MainActor
struct ErrorHandlingViewModelTests {

  // MARK: - Test helper conforming to the protocol

  final class TestViewModel: ErrorHandlingViewModel {
    var error: DomainError?
  }

  // MARK: - handleError with DomainError

  @Test func handleError_withDomainError_setsErrorDirectly() {
    let sut = TestViewModel()

    sut.handleError(DomainError.networkUnavailable)

    #expect(sut.error == .networkUnavailable)
  }

  @Test func handleError_withDomainError_ignoresFallback() {
    let sut = TestViewModel()

    sut.handleError(DomainError.sessionExpired) { .unexpected($0) }

    #expect(sut.error == .sessionExpired)
  }

  // MARK: - handleError with non-DomainError (default fallback)

  @Test func handleError_withGenericError_usesUnexpectedFallback() {
    let sut = TestViewModel()
    struct SomeError: Error, LocalizedError {
      var errorDescription: String? { "boom" }
    }

    sut.handleError(SomeError())

    #expect(sut.error == .unexpected("boom"))
  }

  // MARK: - handleError with non-DomainError (custom fallback)

  @Test func handleError_withGenericError_usesCustomFallback() {
    let sut = TestViewModel()
    struct SomeError: Error, LocalizedError {
      var errorDescription: String? { "network" }
    }

    sut.handleError(SomeError()) { .authenticationFailed($0) }

    #expect(sut.error == .authenticationFailed("network"))
  }

  // MARK: - Overwrites previous error

  @Test func handleError_overwritesPreviousError() {
    let sut = TestViewModel()
    sut.error = .networkUnavailable

    sut.handleError(DomainError.sessionExpired)

    #expect(sut.error == .sessionExpired)
  }
}
