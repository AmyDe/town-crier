import Foundation
import TownCrierDomain

final class SpyOnboardingRepository: OnboardingRepository, @unchecked Sendable {
    var isOnboardingComplete = false
    private(set) var markOnboardingCompleteCallCount = 0

    func markOnboardingComplete() {
        markOnboardingCompleteCallCount += 1
        isOnboardingComplete = true
    }
}
