import Foundation
import Testing
import TownCrierData

@Suite("UserDefaultsOnboardingRepository")
struct UserDefaultsOnboardingRepositoryTests {
    private func makeSUT() -> (UserDefaultsOnboardingRepository, UserDefaults) {
        let defaults = UserDefaults(suiteName: UUID().uuidString)!
        let sut = UserDefaultsOnboardingRepository(defaults: defaults)
        return (sut, defaults)
    }

    @Test("isOnboardingComplete defaults to false")
    func defaultsToFalse() {
        let (sut, _) = makeSUT()

        #expect(!sut.isOnboardingComplete)
    }

    @Test("markOnboardingComplete sets isOnboardingComplete to true")
    func markCompleteSetsTrueFlag() {
        let (sut, _) = makeSUT()

        sut.markOnboardingComplete()

        #expect(sut.isOnboardingComplete)
    }

    @Test("isOnboardingComplete reads from UserDefaults")
    func readsFromUserDefaults() {
        let defaults = UserDefaults(suiteName: UUID().uuidString)!
        defaults.set(true, forKey: "isOnboardingComplete")
        let sut = UserDefaultsOnboardingRepository(defaults: defaults)

        #expect(sut.isOnboardingComplete)
    }

    @Test("markOnboardingComplete persists to UserDefaults")
    func persistsToUserDefaults() {
        let (sut, defaults) = makeSUT()

        sut.markOnboardingComplete()

        #expect(defaults.bool(forKey: "isOnboardingComplete"))
    }
}
