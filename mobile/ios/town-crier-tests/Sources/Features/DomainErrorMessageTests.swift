import Testing
import TownCrierDomain

@Suite("DomainError user messages")
struct DomainErrorMessageTests {
  @Test func networkUnavailable_hasConnectionMessage() {
    let error = DomainError.networkUnavailable
    #expect(error.userTitle == "No Connection")
    #expect(error.userMessage == "Check your internet connection and try again.")
  }

  @Test func sessionExpired_hasAuthMessage() {
    let error = DomainError.sessionExpired
    #expect(error.userTitle == "Session Expired")
    #expect(error.userMessage == "Your session has expired. Please sign in again.")
  }

  @Test func unexpected_hasGenericMessage() {
    let error = DomainError.unexpected("Something broke")
    #expect(error.userTitle == "Something Went Wrong")
    #expect(error.userMessage == "An unexpected error occurred. Please try again.")
  }

  @Test func authenticationFailed_hasAuthMessage() {
    let error = DomainError.authenticationFailed("bad credentials")
    #expect(error.userTitle == "Sign In Failed")
    #expect(error.userMessage == "Unable to sign in. Please try again.")
  }

  @Test func isRetryable_trueForNetwork() {
    #expect(DomainError.networkUnavailable.isRetryable)
  }

  @Test func isRetryable_trueForUnexpected() {
    #expect(DomainError.unexpected("err").isRetryable)
  }

  @Test func isRetryable_trueForSessionExpired() {
    #expect(DomainError.sessionExpired.isRetryable)
  }

  @Test func isRetryable_falseForAuthFailed() {
    #expect(!DomainError.authenticationFailed("err").isRetryable)
  }

  // MARK: - insufficientEntitlement

  @Test func insufficientEntitlement_hasUpgradeTitle() {
    let error = DomainError.insufficientEntitlement(required: "statusChangeAlerts")
    #expect(error.userTitle == "Upgrade Required")
  }

  @Test func insufficientEntitlement_hasUpgradeMessage() {
    let error = DomainError.insufficientEntitlement(required: "statusChangeAlerts")
    #expect(
      error.userMessage
        == "This feature is on a higher plan. Upgrade to use it."
    )
  }

  @Test func insufficientEntitlement_isNotRetryable() {
    let error = DomainError.insufficientEntitlement(required: "statusChangeAlerts")
    #expect(!error.isRetryable)
  }

  @Test func insufficientEntitlement_preservesRequiredField() {
    let error = DomainError.insufficientEntitlement(required: "statusChangeAlerts")
    if case .insufficientEntitlement(let required) = error {
      #expect(required == "statusChangeAlerts")
    } else {
      Issue.record("Expected insufficientEntitlement case")
    }
  }

  // MARK: - serverError

  @Test func serverError_hasServerErrorTitle() {
    let error = DomainError.serverError(statusCode: 500, message: "Internal Server Error")
    #expect(error.userTitle == "Server Error")
  }

  @Test func serverError_hasServerErrorMessage() {
    let error = DomainError.serverError(statusCode: 500, message: "Internal Server Error")
    #expect(error.userMessage == "The server encountered an error. Please try again later.")
  }

  @Test func serverError_isRetryable() {
    #expect(DomainError.serverError(statusCode: 500, message: nil).isRetryable)
  }

  @Test func serverError_preservesStatusCode() {
    let error = DomainError.serverError(statusCode: 400, message: "Bad Request")
    if case .serverError(let statusCode, let message) = error {
      #expect(statusCode == 400)
      #expect(message == "Bad Request")
    } else {
      Issue.record("Expected serverError case")
    }
  }

  @Test func serverError_isNotEqualToNetworkUnavailable() {
    let server = DomainError.serverError(statusCode: 400, message: nil)
    let network = DomainError.networkUnavailable
    #expect(server != network)
  }

  // MARK: - invalidPostcode (tc-y5ee)

  @Test func invalidPostcode_hasSpecificTitle() {
    let error = DomainError.invalidPostcode("NOPE")
    #expect(error.userTitle == "Invalid Postcode")
  }

  @Test func invalidPostcode_hasSpecificMessage() {
    let error = DomainError.invalidPostcode("NOPE")
    #expect(
      error.userMessage
        == "The postcode 'NOPE' doesn't look right. Please enter a valid UK postcode.")
  }

  @Test func invalidPostcode_isNotRetryable() {
    #expect(!DomainError.invalidPostcode("XYZ").isRetryable)
  }

  // MARK: - geocodingFailed (tc-y5ee)

  @Test func geocodingFailed_hasSpecificTitle() {
    let error = DomainError.geocodingFailed("SW1A 1AA")
    #expect(error.userTitle == "Postcode Not Found")
  }

  @Test func geocodingFailed_hasSpecificMessage() {
    let error = DomainError.geocodingFailed("SW1A 1AA")
    #expect(
      error.userMessage
        == "We couldn't find the location for that postcode. Please check and try again.")
  }

  @Test func geocodingFailed_isRetryable() {
    #expect(DomainError.geocodingFailed("SW1A 1AA").isRetryable)
  }

  // MARK: - Custom-shape watch zone boundary (GH#1031, tc-6he3x.7)

  @Test func invalidWatchZoneBoundaryVertexCount_hasInvalidAreaTitle() {
    let error = DomainError.invalidWatchZoneBoundaryVertexCount
    #expect(error.userTitle == "Invalid Area")
    #expect(error.userMessage == "A custom area needs between 3 and 50 points.")
    #expect(!error.isRetryable)
  }

  @Test func invalidWatchZoneBoundaryDuplicateVertex_hasInvalidAreaTitle() {
    let error = DomainError.invalidWatchZoneBoundaryDuplicateVertex
    #expect(error.userTitle == "Invalid Area")
    #expect(error.userMessage == "A custom area can't have two points in the same place.")
    #expect(!error.isRetryable)
  }

  @Test func invalidWatchZoneBoundarySelfIntersecting_hasInvalidAreaTitle() {
    let error = DomainError.invalidWatchZoneBoundarySelfIntersecting
    #expect(error.userTitle == "Invalid Area")
    #expect(
      error.userMessage == "This shape crosses itself. Try drawing it again without crossing lines."
    )
    #expect(!error.isRetryable)
  }

  @Test func invalidWatchZoneBoundaryOutOfBounds_hasInvalidAreaTitle() {
    let error = DomainError.invalidWatchZoneBoundaryOutOfBounds
    #expect(error.userTitle == "Invalid Area")
    #expect(error.userMessage == "A custom area must be drawn within the UK.")
    #expect(!error.isRetryable)
  }

  // MARK: - Watch-zone save errors (tc-9oyhw, GH#1085)

  @Test func invalidWatchZoneBoundaryTooLarge_hasInvalidAreaTitle() {
    let error = DomainError.invalidWatchZoneBoundaryTooLarge
    #expect(error.userTitle == "Invalid Area")
    #expect(error.userMessage == "This area is too large. Try drawing a smaller shape.")
    #expect(!error.isRetryable)
  }

  @Test func watchZoneNameTaken_hasNameAlreadyUsedTitle() {
    let error = DomainError.watchZoneNameTaken
    #expect(error.userTitle == "Name Already Used")
    #expect(
      error.userMessage
        == "You already have a watch zone with this name. Choose a different name.")
    #expect(!error.isRetryable)
  }
}
