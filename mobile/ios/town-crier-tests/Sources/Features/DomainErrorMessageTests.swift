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

    @Test func isRetryable_falseForSessionExpired() {
        #expect(!DomainError.sessionExpired.isRetryable)
    }

    @Test func isRetryable_falseForAuthFailed() {
        #expect(!DomainError.authenticationFailed("err").isRetryable)
    }

    // MARK: - insufficientEntitlement

    @Test func insufficientEntitlement_hasUpgradeTitle() {
        let error = DomainError.insufficientEntitlement(required: "searchApplications")
        #expect(error.userTitle == "Upgrade Required")
    }

    @Test func insufficientEntitlement_hasUpgradeMessage() {
        let error = DomainError.insufficientEntitlement(required: "searchApplications")
        #expect(
            error.userMessage
                == "This feature requires a higher subscription tier. Upgrade to unlock it."
        )
    }

    @Test func insufficientEntitlement_isNotRetryable() {
        let error = DomainError.insufficientEntitlement(required: "searchApplications")
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
}
